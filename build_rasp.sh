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
