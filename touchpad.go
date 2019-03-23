package uinput

import (
	"fmt"
	"io"
	"os"
)

// A TouchPad is an input device that uses absolute axis events, meaning that you can specify
// the exact position the cursor should move to. Therefore, it is necessary to define the size
// of the rectangle in which the cursor may move upon creation of the device.
type TouchPad interface {
	// MoveTo will move the cursor to the specified position on the screen
	MoveTo(x int32, y int32) error

	// LeftClick will issue a single left click.
	LeftClick() error

	// RightClick will issue a right click.
	RightClick() error

	// LeftPress will simulate a press of the left mouse button. Note that the button will not be released until
	// LeftRelease is invoked.
	LeftPress() error

	// LeftRelease will simulate the release of the left mouse button.
	LeftRelease() error

	// RightPress will simulate the press of the right mouse button. Note that the button will not be released until
	// RightRelease is invoked.
	RightPress() error

	// RightRelease will simulate the release of the right mouse button.
	RightRelease() error

	io.Closer
}

type vTouchPad struct {
	name       []byte
	deviceFile *os.File
}

// CreateTouchPad will create a new touch pad device. note that you will need to define the x and y axis boundaries
// (min and max) within which the cursor maybe moved around.
func CreateTouchPad(path string, name []byte, minX int32, maxX int32, minY int32, maxY int32) (TouchPad, error) {
	validateDevicePath(path)
	validateUinputName(name)

	fd, err := createTouchPad(path, name, minX, maxX, minY, maxY)
	if err != nil {
		return nil, err
	}

	return vTouchPad{name: name, deviceFile: fd}, nil
}

func (vTouch vTouchPad) MoveTo(x int32, y int32) error {
	return sendAbsEvent(vTouch.deviceFile, x, y)
}

func (vTouch vTouchPad) LeftClick() error {
	err := sendBtnEvent(vTouch.deviceFile, evBtnLeft, btnStatePressed)
	if err != nil {
		return fmt.Errorf("Failed to issue the LeftClick event: %v", err)
	}

	return sendBtnEvent(vTouch.deviceFile, evBtnLeft, btnStateReleased)
}

func (vTouch vTouchPad) RightClick() error {
	err := sendBtnEvent(vTouch.deviceFile, evBtnRight, btnStatePressed)
	if err != nil {
		return fmt.Errorf("Failed to issue the RightClick event: %v", err)
	}

	return sendBtnEvent(vTouch.deviceFile, evBtnRight, btnStateReleased)
}

// LeftPress will simulate a press of the left mouse button. Note that the button will not be released until
// LeftRelease is invoked.
func (vTouch vTouchPad) LeftPress() error {
	return sendBtnEvent(vTouch.deviceFile, evBtnLeft, btnStatePressed)
}

// LeftRelease will simulate the release of the left mouse button.
func (vTouch vTouchPad) LeftRelease() error {
	return sendBtnEvent(vTouch.deviceFile, evBtnLeft, btnStateReleased)
}

// RightPress will simulate the press of the right mouse button. Note that the button will not be released until
// RightRelease is invoked.
func (vTouch vTouchPad) RightPress() error {
	return sendBtnEvent(vTouch.deviceFile, evBtnRight, btnStatePressed)
}

// RightRelease will simulate the release of the right mouse button.
func (vTouch vTouchPad) RightRelease() error {
	return sendBtnEvent(vTouch.deviceFile, evBtnRight, btnStateReleased)
}

func (vTouch vTouchPad) Close() error {
	return closeDevice(vTouch.deviceFile)
}

func createTouchPad(path string, name []byte, minX int32, maxX int32, minY int32, maxY int32) (fd *os.File, err error) {
	deviceFile, err := createDeviceFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not create absolute axis input device: %v", err)
	}

	err = registerDevice(deviceFile, uintptr(evKey))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register key device: %v", err)
	}
	// register button events (in order to enable left and right click)
	err = ioctl(deviceFile, uiSetKeyBit, uintptr(evBtnLeft))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register left click event: %v", err)
	}
	err = ioctl(deviceFile, uiSetKeyBit, uintptr(evBtnRight))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register right click event: %v", err)
	}

	err = registerDevice(deviceFile, uintptr(evAbs))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register absolute axis input device: %v", err)
	}

	// register x and y axis events
	err = ioctl(deviceFile, uiSetAbsBit, uintptr(absX))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register absolute x axis events: %v", err)
	}
	err = ioctl(deviceFile, uiSetAbsBit, uintptr(absY))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register absolute y axis events: %v", err)
	}

	var absMin [absSize]int32
	absMin[absX] = minX
	absMin[absY] = minY

	var absMax [absSize]int32
	absMax[absX] = maxX
	absMax[absY] = maxY

	return createUsbDevice(deviceFile,
		uinputUserDev{
			Name: toUinputName(name),
			ID: inputID{
				Bustype: busUsb,
				Vendor:  0x4711,
				Product: 0x0817,
				Version: 1},
			Absmin: absMin,
			Absmax: absMax})
}

func sendAbsEvent(deviceFile *os.File, xPos int32, yPos int32) error {
	var ev [2]inputEvent
	ev[0].Type = evAbs
	ev[0].Code = absX
	ev[0].Value = xPos

	ev[1].Type = evAbs
	ev[1].Code = absY
	ev[1].Value = yPos

	for _, iev := range ev {
		buf, err := inputEventToBuffer(iev)
		if err != nil {
			return fmt.Errorf("writing abs event failed: %v", err)
		}

		_, err = deviceFile.Write(buf)
		if err != nil {
			return fmt.Errorf("failed to write abs event to device file: %v", err)
		}
	}

	return syncEvents(deviceFile)
}
