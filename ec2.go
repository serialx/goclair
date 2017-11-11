package goclair

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/dustin/go-humanize"
)

type Instance struct {
	name             string
	privateIpAddress *string
	selected         bool
	launchTime       time.Time
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
	return fmt.Sprintf("%s [%s]", instance.name, humanTime)
}

func (instance Instance) Selected() bool {
	return instance.selected
}

func (instance *Instance) SetSelected(selected bool) {
	instance.selected = selected
}
