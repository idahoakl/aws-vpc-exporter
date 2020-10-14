package subnet

import (
	"context"
	"math"
	"net"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/idahoakl/aws-vpc-exporter/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	Namespace      = "aws_subnet"
	LabelSubnetId  = "subnet_id"
	LabelAZ        = "availability_zone"
	LabelVpcId     = "vpc_id"
	LabelCIDR      = "network_cidr"
	LabelTagPrefix = "tag_"
)

var (
	upOpts = prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "sdk_up",
		Help:      "Connection to AWS SDK successful",
	}
	infoOpts = prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "info",
		Help:      "Info about AWS Subnets",
	}
	availIpOpts = prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "available_ip_addresses",
		Help:      "Count of available IP addresses for a subnet",
	}
	maxIpOpts = prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "maximum_ip_addresses",
		Help:      "Count of maximum IP addresses for a subnet",
	}
)

type collector struct {
	svc     ec2iface.EC2API
	cfg     config.SubnetConfig
	upGauge prometheus.Gauge
}

func New(svc ec2iface.EC2API, cfg config.SubnetConfig) (prometheus.Collector, error) {
	c := collector{
		svc:     svc,
		cfg:     cfg,
		upGauge: prometheus.NewGauge(upOpts),
	}
	return c, nil
}

func (c collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc(
		prometheus.BuildFQName(availIpOpts.Namespace, availIpOpts.Subsystem, availIpOpts.Name),
		availIpOpts.Help,
		[]string{LabelSubnetId},
		nil,
	)
}

func (c collector) Collect(ch chan<- prometheus.Metric) {
	var err error
	availGauge := prometheus.NewGaugeVec(availIpOpts, []string{LabelSubnetId})
	maxGauge := prometheus.NewGaugeVec(maxIpOpts, []string{LabelSubnetId})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filters := make([]*ec2.Filter, 0, 2)
	if len(c.cfg.Filter.Ids) > 0 {
		subnetIdFilter := "subnet-id"
		filters = append(filters, &ec2.Filter{Name: &subnetIdFilter, Values: c.cfg.Filter.Ids})
	}

	if len(c.cfg.Filter.TagFilters) > 0 {
		for _, filter := range c.cfg.Filter.TagFilters {
			if len(filter.Values) > 0 {
				// filter for tag values
				name := "tag:" + filter.Key
				filters = append(filters, &ec2.Filter{Name: &name, Values: filter.Values})
			} else {
				// filter for presence of tag
				name := "tag-key"
				values := []*string{&filter.Key}
				filters = append(filters, &ec2.Filter{Name: &name, Values: values})
			}
		}
	}

	if len(filters) <= 0 {
		log.Error("must include at least 1 filter for subnets, refusing to enumerate all subnets in account")
		return
	}
	input := ec2.DescribeSubnetsInput{
		Filters: filters,
	}

	err = c.svc.DescribeSubnetsPagesWithContext(ctx, &input,
		func(page *ec2.DescribeSubnetsOutput, lastPage bool) bool {
			for _, subnet := range page.Subnets {
				availGauge.WithLabelValues(*subnet.SubnetId).Set(float64(*subnet.AvailableIpAddressCount))

				log.Debugf("subnet: %s\tavailableIps: %d\n", *subnet.SubnetId, *subnet.AvailableIpAddressCount)

				// TODO: error handling for ParseCIDR
				_, network, err := net.ParseCIDR(*subnet.CidrBlock)
				if err == nil {
					ones, bits := network.Mask.Size()
					s := bits - ones

					// five IP addresses within a subnet are reserved by AWS
					// https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#VPC_Sizing
					maxIps := math.Pow(2, float64(s)) - 5
					maxGauge.WithLabelValues(*subnet.SubnetId).Set(maxIps)
					log.Debugf("\tones: %d\tbits:%d\tips: %f\n", ones, bits, maxIps)
				}

				lbls := make([]string, 0, 6)
				lblVals := make([]string, 0, 6)
				if !c.cfg.Info.ExcludeAZ {
					lbls = append(lbls, LabelAZ)
					lblVals = append(lblVals, *subnet.AvailabilityZone)
				}
				if !c.cfg.Info.ExcludeVPC {
					lbls = append(lbls, LabelVpcId)
					lblVals = append(lblVals, *subnet.VpcId)
				}
				if !c.cfg.Info.ExcludeCIDR {
					lbls = append(lbls, LabelCIDR)
					lblVals = append(lblVals, *subnet.CidrBlock)
				}

				if len(c.cfg.Info.IncludeTags) > 0 {
					tagMap := make(map[string]string)
					for _, tag := range subnet.Tags {
						tagMap[*tag.Key] = *tag.Value
					}

					for _, tag := range c.cfg.Info.IncludeTags {
						if t, contains := tagMap[tag]; contains {
							lbls = append(lbls, LabelTagPrefix+tag)
							lblVals = append(lblVals, t)
						}
					}
				}

				if len(lbls) > 0 {
					lbls = append(lbls, LabelSubnetId)
					lblVals = append(lblVals, *subnet.SubnetId)
					ch <- prometheus.MustNewConstMetric(prometheus.NewDesc(
						prometheus.BuildFQName(infoOpts.Namespace, infoOpts.Subsystem, infoOpts.Name),
						infoOpts.Help,
						lbls,
						nil,
					), prometheus.UntypedValue, 1.0, lblVals...)
				}
			}

			return true
		},
	)

	if err != nil {
		log.Error(err)
		c.upGauge.Set(0)
	} else {
		c.upGauge.Set(1)
	}

	c.upGauge.Collect(ch)
	availGauge.Collect(ch)
	maxGauge.Collect(ch)
}
