package sqs

var Regions = map[string]Region{
	APNortheast.Name:  APNortheast,
	APSoutheast.Name:  APSoutheast,
	APSoutheast2.Name: APSoutheast2,
	EUWest.Name:       EUWest,
	USEast.Name:       USEast,
	USWest.Name:       USWest,
	USWest2.Name:      USWest2,
	SAEast.Name:       SAEast,
}

// Pre-defined regions
// http://docs.aws.amazon.com/general/latest/gr/rande.html#sqs_region

var USEast = Region{
	"us-east-1",
	"https://sqs.us-east-1.amazonaws.com",
}

var USWest = Region{
	"us-west-1",
	"https://sqs.us-west-1.amazonaws.com",
}

var USWest2 = Region{
	"us-west-2",
	"https://sqs.us-west-2.amazonaws.com",
}

var EUWest = Region{
	"eu-west-1",
	"https://sqs.eu-west-1.amazonaws.com",
}

var APSoutheast = Region{
	"ap-southeast-1",
	"https://sqs.ap-southeast-1.amazonaws.com",
}

var APSoutheast2 = Region{
	"ap-southeast-2",
	"https://sqs.ap-southeast-2.amazonaws.com",
}

var APNortheast = Region{
	"ap-northeast-1",
	"https://sqs.ap-northeast-1.amazonaws.com",
}

var SAEast = Region{
	"sa-east-1",
	"https://sqs.sa-east-1.amazonaws.com",
}
