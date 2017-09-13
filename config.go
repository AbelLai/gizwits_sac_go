package gizwits_sac_go

type Config struct {
	Addr    string
	Retry   int
	Timeout struct {
		Connect int
		Read    int
		Write   int
	}
	PrefetchCount int
	AuthDataList  []AuthData
	Callbacks     struct {
		Read  func(string)
		Write func() T
	}
	Logger ILogger
}

// "auth_data": {
//   "product_key": "xxxxx",
//   "auth_id": "xxxxxx",
//   "auth_secret": "xxxxx",
//   "subkey": "xxxxx",
//   "events": ["event name"]
// }
type AuthData struct {
	ProductKey string   `json:"product_key"`
	AuthId     string   `json:"auth_id"`
	AuthSecret string   `json:"auth_secret"`
	Subkey     string   `json:"subkey"`
	Events     []string `json:"events"`
}
