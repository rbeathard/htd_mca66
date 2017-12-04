# htd_mca66
Serial to MQTT gateway for mca66 whole-house audio system from Home Theater Direct (http://htd.com)


I started this project because I had issue with the GW-SL1 serial-ethernet gateway occasionally locking up. I also leveraged the development to learn MQTT and Home Assistant. Originally developed in node.js but quickly switched to golang because it was easier to maintain the dependencies between development (Mac laptop) and target (Raspberry PI) machines. Nothing beats scp'ing an executable.

The serial interface to the MCA-66 audio system is a bit orientated protocol which made it challenging in parsing. I have included the two HTD documentation that describes the protocol.

The program is broken into two components:
* htd - main program, MQTT handling
* htdserial - main package used to handle serial communication with MCA-66

Project utilizes two external packages:
  * github.com/eclipse/paho.mqtt.golang - mqtt library
  * github.com/jacobsa/go-serial/serial - serial library

#### To build
The following will cross compile the code for raspberry pi on either a mac or linux developmental environment. It is assumed that you already have golang installed.
1. Download the project
2. Navigate to root of project
3. Run build.sh bash script

build.sh command will pull in the required dependencies and cross compile for raspberry pi.
```
export GOPATH=$PWD
export GOOS=linux
export GOARCH=arm
export GOARM=5
if [ ! -d src/github.com/jacobsa/go-serial/serial ] ; then
    # just get the package
    go get -d github.com/jacobsa/go-serial/serial
fi
if [ ! -d github.com/eclipse/paho.mqtt.golang ] ; then
    # just get the package
    go get -d github.com/eclipse/paho.mqtt.golang
fi
go build htdserial
go build htd
```

#### To Run
*Notes: hardcoded to use /deb/ttyUSB0*
```
.htd
````
