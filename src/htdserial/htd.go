package htdserial

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jacobsa/go-serial/serial"
)

// ZoneStatusHandler -
type ZoneStatusHandler func(ZoneStatusMsg)

// ZoneStateHandler -
type ZoneStateHandler func(ZoneStateMsg)

// Serial - holds the information neccessary to service htd *Serial port
type Serial struct {
	serialPort        string
	readerExit        chan bool
	connectorFin      chan bool
	zoneStatusEvent   chan ZoneStatusMsg
	zoneStateEvent    chan ZoneStateMsg
	cmdReq            chan bytes.Buffer
	serialFd          io.ReadWriteCloser
	currentMsg        bytes.Buffer
	zoneStatusHandler ZoneStatusHandler
	zoneStateHandler  ZoneStateHandler
}

// ZoneStatusMsg -
type ZoneStatusMsg struct {
	Zone       string
	Power      string
	Mute       string
	PartyMode  string
	PartyInput string
	Input      string
	Volume     string
	Treble     string
	Bass       string
	Balance    string
}

// ZoneStateMsg -
type ZoneStateMsg struct {
	ZoneState   [6]string
	KeypadState [6]string
}

//NewSerial - create a new htd *Serial Context
func NewSerial(_serialPort string,
	zoneStateHandler ZoneStateHandler,
	zoneStatusHandler ZoneStatusHandler) *Serial {
	return (&Serial{
		serialPort:        _serialPort,
		currentMsg:        bytes.Buffer{},
		readerExit:        make(chan bool),
		connectorFin:      make(chan bool),
		zoneStatusEvent:   make(chan ZoneStatusMsg),
		zoneStateEvent:    make(chan ZoneStateMsg),
		cmdReq:            make(chan bytes.Buffer),
		zoneStateHandler:  zoneStateHandler,
		zoneStatusHandler: zoneStatusHandler})
}

func (htd *Serial) startReader() {
	i := 0
	for {
		buf := make([]byte, 1)
		_, err := htd.serialFd.Read(buf)
		i = i + 1
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from serial port: %s\n", err)
				break
			}
			htd.readerExit <- false
			break
		} else {
			htd.processRead(buf)
		}
	}
}

// Start - Start the serial processing..
func (htd *Serial) Start() {
	connected := false

	go htd.startConnector()
	go func() {
		for {
			select {
			case response := <-htd.connectorFin:
				if response == true {
					connected = true
					if htd.serialFd == nil {
						fmt.Printf("hanlder serialFd == nil\n")
					}
					go htd.startReader()
					go htd.AllZoneQuery()
				} else {
					// throttle failed connection retries
					time.Sleep(1000 * time.Millisecond)
					fmt.Printf("Reconnecting...\n")
					go htd.startConnector()
					fmt.Printf("Started Reconnector\n")
				}
			case <-htd.readerExit:
				if connected {
					connected = false
					htd.serialFd.Close()
				}
				// reconnect
				go htd.startConnector()

			case c := <-htd.zoneStatusEvent:
				go htd.zoneStatusHandler(c)
				//				b, _ := json.Marshal(c)
				//fmt.Printf("zone status: %s\n", string(b))
			case c := <-htd.zoneStateEvent:
				go htd.zoneStateHandler(c)
				//				b, _ := json.Marshal(c)
				//fmt.Printf("zone status: %s\n", string(b))

			case c := <-htd.cmdReq:
				if connected == true {
					fmt.Printf("Write Command: %s\n", hex.EncodeToString(c.Bytes()))
					if htd.serialFd == nil {
						fmt.Printf("serialFD is nil\n")
					}
					_, err := htd.serialFd.Write(c.Bytes())
					if err != nil {
						fmt.Printf("write failure: %s\n", err)
						connected = false
						// wakeup reader by closing FD...
						htd.serialFd.Close()
					}
				} // else send not connected event.
			}
		}

	}()
}

func (htd *Serial) startConnector() {
	var err error

	// connect
	serialOptions := serial.OpenOptions{
		PortName:               htd.serialPort,
		BaudRate:               38400,
		DataBits:               8,
		StopBits:               1,
		MinimumReadSize:        1,
		InterCharacterTimeout:  0,
		ParityMode:             serial.PARITY_NONE,
		Rs485Enable:            false,
		Rs485RtsHighDuringSend: false,
		Rs485RtsHighAfterSend:  false,
	}
	fmt.Printf("Connector Start...\n")

	htd.serialFd, err = serial.Open(serialOptions)
	if htd.serialFd == nil {
		fmt.Printf("connector serialFD is nil\n")
	}
	if err != nil {
		fmt.Printf("Error opening serial port: %s\n", err)
		htd.connectorFin <- false
		fmt.Printf("Sent Fin\n")
	} else {
		fmt.Printf("Open success\n")
		htd.connectorFin <- true
	}
}

func (htd *Serial) processRead(newRawData []byte) {

	var cmd byte
	var sizeOfMsg int

	// Pick up where we left off with any data left over from last time
	htd.currentMsg.WriteByte(newRawData[0])

	// Make sure we are at the start of a packet, if not go to next byte
	if htd.currentMsg.Len() == 1 && htd.currentMsg.Bytes()[0] != 0x02 {
		fmt.Printf("Not head byte: %s\n", hex.EncodeToString(newRawData))
		htd.currentMsg.Reset()
		return
	}

	// Do we not have enough to read the command byte? Then save the rest for next time.
	if htd.currentMsg.Len() < 4 {
		return
	}

	// Determine command packet sizeOfMsggth based on type
	cmd = htd.currentMsg.Bytes()[3]
	switch cmd {
	case 0x05: // Zone internal status
		sizeOfMsg = 4 + 9 + 1
		break
	case 0x06: // Audio and Keypad Exist channel
		sizeOfMsg = 4 + 9 + 1
		break
	// The following commands should never be received.
	case 0x04: // Zone modification command
		sizeOfMsg = 4 + 1 + 1
		break
	case 0x08: // read Model Info
		sizeOfMsg = 6
		break
	case 0x0A: // reqd memory at M1, M2 or M3
		sizeOfMsg = 6
		break
	case 0x0B: // response to M1, M2 or M3
		sizeOfMsg = 6
		break
	default:
		sizeOfMsg = 1
		fmt.Printf("Unknown response code: %s\n", hex.EncodeToString(newRawData))
		break
	}

	// Not enough data to form a complete packet? Save for next time.
	if htd.currentMsg.Len() == sizeOfMsg {
		if htd.currentMsg.Bytes()[3] == 0x05 {
			// zone internal status
			htd.processZoneStatus(htd.currentMsg.Bytes())
		} else if htd.currentMsg.Bytes()[3] == 0x06 {
			htd.processZoneStates(htd.currentMsg.Bytes())
		}
		htd.currentMsg.Reset()
		return
	}
	return
}

func (htd *Serial) processZoneStates(currentMsg []byte) {
	var zoneState ZoneStateMsg

	for i := 0; i < 6; i++ {
		zoneState.ZoneState[i] = htd.bitTest(uint(i), currentMsg[5])
	}

	for i := 0; i < 6; i++ {
		zoneState.KeypadState[i] = htd.bitTest(uint(i), currentMsg[6])
	}
	htd.zoneStateEvent <- zoneState

}

func (htd *Serial) processZoneStatus(currentMsg []byte) {
	var zoneStatus ZoneStatusMsg
	dataStart := 3
	if currentMsg[dataStart+4] != 0 {
		//fmt.Printf("processZoneStatus: known issue with message.\n")
		return
	}

	zoneStatus.Zone = strconv.Itoa(int(currentMsg[2]))
	data1 := currentMsg[dataStart+1]
	zoneStatus.Power = htd.bitTest(7, data1)
	zoneStatus.Mute = htd.bitTest(6, data1)
	zoneStatus.PartyMode = htd.bitTest(3, data1)
	zoneStatus.PartyInput = strconv.Itoa(int(data1 & byte(0x3)))
	zoneStatus.Input = strconv.Itoa(int(currentMsg[dataStart+5]) + 1)
	volume := currentMsg[dataStart+6]
	volume = 0xff - volume + 1
	zoneStatus.Volume = strconv.Itoa(int(volume))

	treble := currentMsg[dataStart+7]
	zoneStatus.Treble = htd.trebleBass(treble)

	bass := currentMsg[dataStart+8]
	zoneStatus.Bass = htd.trebleBass(bass)

	balance := currentMsg[dataStart+9]
	zoneStatus.Balance = htd.balanceLevel(balance)
	htd.zoneStatusEvent <- zoneStatus
}

func (htd *Serial) balanceLevel(byteValue byte) string {
	switch byteValue {
	case 0xee:
		return "-18"
	case 0xf4:
		return "-12"
	case 0xfa:
		return "-6"
	case 0x00:
		return "0"
	case 0x06:
		return "6"
	case 0x0c:
		return "12"
	case 0x12:
		return "18"
	}
	return "err"
}

func (htd *Serial) trebleBass(byteValue byte) string {
	switch byteValue {
	case 0xf4:
		return "-12"
	case 0xf8:
		return "-8"
	case 0xfc:
		return "-4"
	case 0x00:
		return "0"
	case 0x04:
		return "4"
	case 0x08:
		return "8"
	case 0x0c:
		return "12"
	}
	return "Err"
}

func (htd *Serial) bitTest(pos uint, byteValue byte) string {
	val := byteValue & (1 << pos)
	if val > 0 {
		return "on"
	}
	return "off"
}

// PowerOn - Turn power on to a zone
func (htd *Serial) PowerOn(zone int) {
	htd.sendCmd(zone, 0x20)
}

// PowerOff - Turn power off to a zone
func (htd *Serial) PowerOff(zone int) {
	htd.sendCmd(zone, 0x21)
}

// AllOn - Turn all zones on
func (htd *Serial) AllOn() {
	htd.sendCmd(0, 0x38)
}

// AllOff - Turn all zones off
func (htd *Serial) AllOff() {
	htd.sendCmd(0, 0x39)
}

// VolumeUp -
func (htd *Serial) VolumeUp(zone int) {
	htd.sendCmd(zone, 0x09)
}

// VolumeDown -
func (htd *Serial) VolumeDown(zone int) {
	htd.sendCmd(zone, 0x0A)
}

// BalanceLeft -
func (htd *Serial) BalanceLeft(zone int) {
	htd.sendCmd(zone, 0x2B)
}

// BalanceRight -
func (htd *Serial) BalanceRight(zone int) {
	htd.sendCmd(zone, 0x2A)
}

// TrebleUp -
func (htd *Serial) TrebleUp(zone int) {
	htd.sendCmd(zone, 0x28)
}

// TrebleDown -
func (htd *Serial) TrebleDown(zone int) {
	htd.sendCmd(zone, 0x29)
}

// BassUp -
func (htd *Serial) BassUp(zone int) {
	htd.sendCmd(zone, 0x26)
}

// BassDown -
func (htd *Serial) BassDown(zone int) {
	htd.sendCmd(zone, 0x27)
}

// SetSource -
func (htd *Serial) SetSource(zone int, source int) {
	var byteSource byte
	byteSource = byte(source + 2)
	htd.sendCmd(zone, byteSource)
}

// AllZoneQuery -
func (htd *Serial) AllZoneQuery() {
	htd.ZoneQuery(0)
}

// ZoneQuery -
func (htd *Serial) ZoneQuery(zone int) {
	htd.sendCmd(zone, 0)
}
func (htd *Serial) sendCmd(zone int, cmd byte) {
	var message bytes.Buffer
	message.WriteByte(0x02)
	message.WriteByte(0x00)
	message.WriteByte(byte(zone & 0xff))
	if cmd == 0x00 {
		// request message
		message.WriteByte(0x06)
	} else {
		message.WriteByte(0x04)
	}
	message.WriteByte(cmd)
	checksum := htd.calcChecksum(message.Bytes())
	message.WriteByte(checksum)
	htd.cmdReq <- message
}

func (htd *Serial) calcChecksum(message []byte) byte {
	var checksum byte
	checksum = 0x00
	for index := 0; index < len(message); index++ {
		checksum += message[index]
	}
	checksum &= 0xFF
	return checksum
}
