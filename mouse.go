package uinput

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// A Mouse is a device that will trigger an absolute change event.
// For details see: https://www.kernel.org/doc/Documentation/input/event-codes.txt
type Mouse interface {
	// MoveLeft will move the mouse cursor left by the given number of pixel.
	MoveLeft(pixel int32) error

	// MoveRight will move the mouse cursor right by the given number of pixel.
	MoveRight(pixel int32) error

	// MoveUp will move the mouse cursor up by the given number of pixel.
	MoveUp(pixel int32) error

	// MoveDown will move the mouse cursor down by the given number of pixel.
	MoveDown(pixel int32) error

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

type vMouse struct {
	name       []byte
	deviceFile *os.File
}

// CreateMouse will create a new mouse input device. A mouse is a device that allows relative input.
// Relative input means that all changes to the x and y coordinates of the mouse pointer will be
func CreateMouse(path string, name []byte) (Mouse, error) {
	validateDevicePath(path)
	validateUinputName(name)

	fd, err := createMouse(path, name)
	if err != nil {
		return nil, err
	}

	return vMouse{name: name, deviceFile: fd}, nil
}

// MoveLeft will move the cursor left by the number of pixel specified.
func (vRel vMouse) MoveLeft(pixel int32) error {
	return sendRelEvent(vRel.deviceFile, relX, -pixel)
}

// MoveRight will move the cursor right by the number of pixel specified.
func (vRel vMouse) MoveRight(pixel int32) error {
	return sendRelEvent(vRel.deviceFile, relX, pixel)
}

// MoveUp will move the cursor up by the number of pixel specified.
func (vRel vMouse) MoveUp(pixel int32) error {
	return sendRelEvent(vRel.deviceFile, relY, -pixel)
}

// MoveDown will move the cursor down by the number of pixel specified.
func (vRel vMouse) MoveDown(pixel int32) error {
	return sendRelEvent(vRel.deviceFile, relY, pixel)
}

// LeftClick will issue a LeftClick.
func (vRel vMouse) LeftClick() error {
	err := sendBtnEvent(vRel.deviceFile, evBtnLeft, btnStatePressed)
	if err != nil {
		return fmt.Errorf("Failed to issue the LeftClick event: %v", err)
	}

	return sendBtnEvent(vRel.deviceFile, evBtnLeft, btnStateReleased)
}

// RightClick will issue a RightClick
func (vRel vMouse) RightClick() error {
	err := sendBtnEvent(vRel.deviceFile, evBtnRight, btnStatePressed)
	if err != nil {
		return fmt.Errorf("Failed to issue the RightClick event: %v", err)
	}

	return sendBtnEvent(vRel.deviceFile, evBtnRight, btnStateReleased)
}

// LeftPress will simulate a press of the left mouse button. Note that the button will not be released until
// LeftRelease is invoked.
func (vRel vMouse) LeftPress() error {
	return sendBtnEvent(vRel.deviceFile, evBtnLeft, btnStatePressed)
}

// LeftRelease will simulate the release of the left mouse button.
func (vRel vMouse) LeftRelease() error {
	return sendBtnEvent(vRel.deviceFile, evBtnLeft, btnStateReleased)
}

// RightPress will simulate the press of the right mouse button. Note that the button will not be released until
// RightRelease is invoked.
func (vRel vMouse) RightPress() error {
	return sendBtnEvent(vRel.deviceFile, evBtnRight, btnStatePressed)
}

// RightRelease will simulate the release of the right mouse button.
func (vRel vMouse) RightRelease() error {
	return sendBtnEvent(vRel.deviceFile, evBtnRight, btnStateReleased)
}

// Close closes the device and releases the device.
func (vRel vMouse) Close() error {
	return closeDevice(vRel.deviceFile)
}

func createMouse(path string, name []byte) (fd *os.File, err error) {
	deviceFile, err := createDeviceFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not create relative axis input device: %v", err)
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

	err = registerDevice(deviceFile, uintptr(evRel))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register relative axis input device: %v", err)
	}

	// register x and y axis events
	err = ioctl(deviceFile, uiSetRelBit, uintptr(relX))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register relative x axis events: %v", err)
	}
	err = ioctl(deviceFile, uiSetRelBit, uintptr(relY))
	if err != nil {
		deviceFile.Close()
		return nil, fmt.Errorf("failed to register relative y axis events: %v", err)
	}

	return createUsbDevice(deviceFile,
		uinputUserDev{
			Name: toUinputName(name),
			ID: inputID{
				Bustype: busUsb,
				Vendor:  0x4711,
				Product: 0x0816,
				Version: 1}})
}

func sendRelEvent(deviceFile *os.File, eventCode uint16, pixel int32) error {
	iev := inputEvent{
		Time:  syscall.Timeval{Sec: 0, Usec: 0},
		Type:  evRel,
		Code:  eventCode,
		Value: pixel}

	buf, err := inputEventToBuffer(iev)
	if err != nil {
		return fmt.Errorf("writing abs event failed: %v", err)
	}

	_, err = deviceFile.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write rel event to device file: %v", err)
	}

	return syncEvents(deviceFile)
}
