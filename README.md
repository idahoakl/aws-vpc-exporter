# AWS VPC Exporter
This service expose details and state about AWS VPC objects via HTTP endpoints for consumption by Prometheus.

Currently only information about subnets are exposed but other objects may be added in the future.

## Build
Currently using Go 1.14 with Go Modules for dependency management.
### Docker image
`docker build -t aws-vpc-exporter:dev .`

### Local
`CGO_ENABLED=0 go build aws-vpc-exporter.go`

## Run
`./aws-vpc-exporter --subnetIds subnet-1234,subnet-abcd`

## AWS SDK configuration
The AWS SDK for Golang v1 is used to provide connectivity to AWS.  Configuration for AWS region and credentials are
provided via the environment variables that the AWS SDK for Golang defines.

Required AWS permissions are detailed below for each type of object that can be collected from.

### Subnet objects
Name|Link
---|---
`ec2.DescribeSubnets`|https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html


## Exporter configuration
The exporter uses a config file to drive the configuration of what objects to collect metrics for and what details about the objects
should be exposed.  See [config.yaml](config.yaml) for an example configuration.

By default details about collected objects are exposed in info metrics.  This can be disabled to prevent leakage of VPC configuration if desired.

**NOTE: The exporter will refuse to collect and expose data on all VPC related objects within an AWS account.  An appropriate filter
must be provided for each object type.**  
 
## Exposed metrics
The exporter exposes an info metric for tag level details about AWS VPC objects with separate metrics for the state of objects.

The intention of using an info metric is to reduce cardinality of metrics within Prometheus as the number of metrics for each object
increases.

### Subnet objects
Name|Description|Labels
---|---|---
aws_subnet_sdk_up|Health metric if exporter is able to successfully describe subnets|
aws_subnet_info|Ancillary VPC information pertaining to a subnet|`subnet_id`, `availability_zone`, `vpc_id`, `network_cidr`, included user tags with the format `tag_usertag`
aws_subnet_available_ip_addresses|Count of available IP addresses|`subnet_id`
aws_subnet_maximum_ip_addresses|Maximum possible IP addresses, computed from subnet CIDR.  AWS ip address reservation is taken into account.  See https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#VPC_Sizing|`subnet_id`
