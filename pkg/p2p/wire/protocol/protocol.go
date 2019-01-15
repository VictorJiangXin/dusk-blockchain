package protocol

// ServiceFlag indicates the services provided by the Node.
type ServiceFlag uint64

const (
	// ProtocolVersion is the current protocol version
	ProtocolVersion uint32 = 10000

	// UserAgent is a basic user agent token in the following format: /Name:Version/Name:Version/.../
	UserAgent = "/Dusk:1.0/GO:1.11/" //TODO: Think of a better token name

	// NodePeerService indicates the Node's type
	NodePeerService ServiceFlag = 1
	// BloomFilerService ServiceFlag = 2 // Not implemented
	// PrunedNode        ServiceFlag = 3 // Not implemented
	// LightNode         ServiceFlag = 4 // Not implemented
)

// Magic is the network that Dusk is running on
type Magic uint32

const (
	// MainNet identifies the production network of the Dusk blockchain
	MainNet Magic = 0x7630401f
	// TestNet identifies the test network of the Dusk blockchain
	TestNet Magic = 0x74746e41
	// DevNet identifies the development network of the Dusk blockchain
	DevNet Magic = 0x74736e40
)