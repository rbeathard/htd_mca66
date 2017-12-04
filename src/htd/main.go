package main

import (
	"htdserial"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

func processZoneCmd(client mqtt.Client, message mqtt.Message) {
	levels := strings.Split(message.Topic(), "/")
	if len(levels) != 5 {
		fmt.Printf("Unknown message on topic: %s, payload: %s, len: %d\n", message.Topic(), message.Payload(), len(levels))
		return
	}
	zone, err := strconv.Atoi(levels[3])
	if err != nil {
		fmt.Printf("Unknown message on topic: %s, payload: %s, level: %s\n", message.Topic(), message.Payload(), levels[3])
		return
	}
	fmt.Printf("%s\n", levels[4])
	switch levels[4] {
	case "poweroff":
		go serial.PowerOff(zone)
	case "poweron":
		go serial.PowerOn(zone)
	case "volumeup":
		go serial.VolumeUp(zone)
	case "volumedown":
		go serial.VolumeDown(zone)
	case "balanceleft":
		go serial.BalanceLeft(zone)
	case "balanceright":
		go serial.BalanceRight(zone)
	case "trebleup":
		go serial.TrebleUp(zone)
	case "trebledown":
		go serial.TrebleDown(zone)
	case "bassup":
		go serial.BassUp(zone)
	case "bassdown":
		go serial.BassDown(zone)
	case "source":
		source, err := strconv.Atoi(string(message.Payload()[:]))
		if err != nil {
			fmt.Printf("Source value not defined or not integeter. topic: %s, payload: %s\n", message.Topic(), message.Payload())
			return
		}
		if source < 1 || source > 6 {
			fmt.Printf("Source value is out of range. topic: %s, payload: %s\n", message.Topic(), message.Payload())
			return
		}
		go serial.SetSource(zone, source)
	case "query":
		go serial.ZoneQuery(zone)
	default:
		fmt.Printf("Unknown message on topic: %s, payload: %s\n", message.Topic(), message.Payload())
		return
	}
}

func zoneStatusHandler(zoneStatusMsg htdserial.ZoneStatusMsg) {
	topic := "/htdserial/status/" + string(zoneStatusMsg.Zone) + "/"
	mqttClient.Publish(topic+"power", byte(0), false, zoneStatusMsg.Power)
	mqttClient.Publish(topic+"volume", byte(0), false, zoneStatusMsg.Volume)
	mqttClient.Publish(topic+"source", byte(0), false, zoneStatusMsg.Input)
	mqttClient.Publish(topic+"bass", byte(0), false, zoneStatusMsg.Bass)
	mqttClient.Publish(topic+"treble", byte(0), false, zoneStatusMsg.Treble)
	mqttClient.Publish(topic+"balance", byte(0), false, zoneStatusMsg.Balance)
	mqttClient.Publish(topic+"mute", byte(0), false, zoneStatusMsg.Mute)
	mqttClient.Publish(topic+"party-mode", byte(0), false, zoneStatusMsg.PartyMode)
	mqttClient.Publish(topic+"party-input", byte(0), false, zoneStatusMsg.PartyInput)
	b, _ := json.Marshal(zoneStatusMsg)
	fmt.Printf("zone status: %s\n", string(b))
}

func zoneStateHandler(zoneStateMsg htdserial.ZoneStateMsg) {
	b, _ := json.Marshal(zoneStateMsg)
	fmt.Printf("zone status: %s\n", string(b))
}

var mqttClient mqtt.Client
var serial *htdserial.Serial

func main() {
	serial = htdserial.NewSerial("/dev/ttyUSB0", zoneStateHandler, zoneStatusHandler)
	serial.Start()

	opts := mqtt.NewClientOptions().AddBroker("tcp://192.168.1.200:1883").SetClientID("htdserial_serial")
	opts.SetKeepAlive(2 * time.Second)
	//opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)
	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe("/htdserial/set/#", byte(0), processZoneCmd); token.Wait() && token.Error() != nil && token.Error() != mqtt.ErrNotConnected {
			panic(token.Error())
		}
	}
	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	for {
		time.Sleep(5000 * time.Minute)
	}
}
