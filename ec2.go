package goclair

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Instance struct {
	name               string
	privateIpAddress   *string
	selected           bool
	launchTime         time.Time
	connectableLock    sync.Mutex
	connectableChecked bool
	connectable        bool
}

type ByName []*Instance

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return strings.ToLower(a[i].name) < strings.ToLower(a[j].name) }

func GetInstances() []*Instance {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable, // Use $HOME/.aws/config
	}))

	ec2Svc := ec2.New(sess)

	output, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		panic(fmt.Sprintf("DescribeInstances error: %v", err))
	}
	// TODO(serialx): If nextToken != nil we need to do pagination

	instances := make([]*Instance, 0)

	for _, reservation := range output.Reservations {
		for _, instance := range reservation.Instances {
			name := *instance.InstanceId
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" && len(*tag.Value) > 0 {
					name = *tag.Value
				}
			}
			inst := &Instance{
				name:             name,
				privateIpAddress: instance.PrivateIpAddress,
				launchTime:       *instance.LaunchTime,
			}
			instances = append(instances, inst)
		}
	}
	sort.Sort(ByName(instances))
	return instances
}

func (instance *Instance) Label() string {
	humanTime := humanize.Time(instance.launchTime)
	humanTime = strings.Replace(humanTime, " ago", "", 1)
	tags := ""
	if instance.connectable {
		tags += " ✓"
	} else if instance.connectableChecked && !instance.connectable {
		tags += " x"
	}
	return fmt.Sprintf("%s [%s]%s", instance.name, humanTime, tags)
}

func (instance Instance) Selected() bool {
	return instance.selected
}

func (instance *Instance) SetSelected(selected bool) {
	instance.selected = selected
}

func (instance *Instance) CheckConnectivity(callback func(instance *Instance)) {
	if instance.privateIpAddress != nil && !instance.connectable {
		go func() {
			instance.connectableLock.Lock()
			defer instance.connectableLock.Unlock()

			config := &ssh.ClientConfig{
				User:            "ubuntu",
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}

			authSock := os.Getenv("SSH_AUTH_SOCK")
			// There's SSH Agent present
			if len(authSock) > 0 {
				conn, err := net.Dial("unix", authSock)
				if err == nil {
					agentClient := agent.NewClient(conn)
					config.Auth = []ssh.AuthMethod{
						ssh.PublicKeysCallback(agentClient.Signers),
					}
				}
			}

			client, err := ssh.Dial("tcp", *instance.privateIpAddress+":22", config)
			if err != nil {
				// XXX(serialx): I hate golang error type system
				if strings.HasPrefix(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
					instance.connectable = true
					instance.connectableChecked = true
					callback(instance)
					return
				} else {
					panic(err)
					instance.connectableChecked = true
					callback(instance)
					return
				}
			}
			client.Conn.Close()
		}()
	}
}

func (instance *Instance) ConnectCommand() (string, error) {
	if instance.privateIpAddress != nil && instance.connectable {
		addr := *instance.privateIpAddress
		return fmt.Sprintf("ssh -p %d %s@%s", 22, "ubuntu", addr), nil
	}
	return "", errors.New("No connection info")
}
