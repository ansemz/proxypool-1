package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ssrlive/proxypool/pkg/tool"
)

var (
	ErrorNotSSRLink             = errors.New("not a correct ssr link")
	ErrorPasswordParseFail      = errors.New("password parse failed")
	ErrorPathNotComplete        = errors.New("path not complete")
	ErrorMissingQuery           = errors.New("link missing query")
	ErrorProtocolParamParseFail = errors.New("protocol param parse failed")
	ErrorObfsParamParseFail     = errors.New("obfs param parse failed")
	ErrorOtDomainParseFail      = errors.New("ot_domain parse failed")
	ErrorOtPathParseFail        = errors.New("ot_path parse failed")
)

// 字段依据clash的配置设计
type ShadowsocksR struct {
	Base
	Password      string `yaml:"password" json:"password"`
	Cipher        string `yaml:"cipher" json:"cipher"`
	Protocol      string `yaml:"protocol" json:"protocol"`
	ProtocolParam string `yaml:"protocol-param,omitempty" json:"protocol-param,omitempty"`
	Obfs          string `yaml:"obfs" json:"obfs"`
	ObfsParam     string `yaml:"obfs-param,omitempty" json:"obfs-param,omitempty"`
	Ot_enable     int    `yaml:"ot_enable,omitempty" json:"ot_enable,omitempty"`
	Ot_domain     string `yaml:"ot_domain,omitempty" json:"ot_domain,omitempty"`
	Ot_path       string `yaml:"ot_path,omitempty" json:"ot_path,omitempty"`
}

func (ssr ShadowsocksR) Identifier() string {
	return net.JoinHostPort(ssr.Server, strconv.Itoa(ssr.Port)) + ssr.Password + ssr.ProtocolParam
}

func (ssr ShadowsocksR) String() string {
	data, err := json.Marshal(ssr)
	if err != nil {
		return ""
	}
	return string(data)
}

func (ssr ShadowsocksR) ToClash() string {
	theString := ssr.String()
	if len(theString) > 0 {
		return "- " + theString
	} else {
		return ""
	}
}

func (ssr ShadowsocksR) ToSurge() string {
	return ""
}

func (ssr ShadowsocksR) Clone() Proxy {
	return &ssr
}

// https://github.com/HMBSbige/ShadowsocksR-Windows/wiki/SSR-QRcode-scheme
func (ssr ShadowsocksR) Link() (link string) {
	payload := fmt.Sprintf("%s:%d:%s:%s:%s:%s",
		ssr.Server, ssr.Port, ssr.Protocol, ssr.Cipher, ssr.Obfs, tool.Base64EncodeString(ssr.Password, true))
	query := url.Values{}
	query.Add("obfsparam", tool.Base64EncodeString(ssr.ObfsParam, true))
	query.Add("protoparam", tool.Base64EncodeString(ssr.ProtocolParam, true))
	//query.Add("remarks", tool.Base64EncodeString(ssr.Name, true))
	query.Add("group", tool.Base64EncodeString("proxypoolss.herokuapp.com", true))
	if ssr.Ot_enable != 0 {
		query.Add("ot_enable", "1")
		query.Add("ot_domain", tool.Base64EncodeString(ssr.Ot_domain, true))
		query.Add("ot_path", tool.Base64EncodeString(ssr.Ot_path, true))
	}
	payload = tool.Base64EncodeString(fmt.Sprintf("%s/?%s", payload, query.Encode()), true)
	return fmt.Sprintf("ssr://%s", payload)
}

func ParseSSRLink(link string) (*ShadowsocksR, error) {
	if !strings.HasPrefix(link, "ssr") {
		return nil, ErrorNotSSRLink
	}

	ssrmix := strings.SplitN(link, "://", 2)
	if len(ssrmix) < 2 {
		return nil, ErrorNotSSRLink
	}
	linkPayloadBase64 := ssrmix[1]
	payload, err := tool.Base64DecodeString(linkPayloadBase64)
	if err != nil {
		return nil, ErrorMissingQuery
	}

	infoPayload := strings.SplitN(payload, "/?", -1)
	if len(infoPayload) > 2 {
		return nil, ErrorNotSSRLink
	}
	ssrpath := strings.Split(infoPayload[0], ":")
	if len(ssrpath) < 6 {
		return nil, ErrorPathNotComplete
	}
	// base info
	server := strings.ToLower(ssrpath[0])
	port, _ := strconv.Atoi(ssrpath[1])
	protocol := strings.ToLower(ssrpath[2])
	cipher := strings.ToLower(ssrpath[3])
	obfs := strings.ToLower(ssrpath[4])
	password, err := tool.Base64DecodeString(ssrpath[5])
	if err != nil {
		return nil, ErrorPasswordParseFail
	}

	if len(infoPayload) == 1 {
		return &ShadowsocksR{
			Base: Base{
				Name:   "",
				Server: server,
				Port:   port,
				Type:   "ssr",
			},
			Password:      password,
			Cipher:        cipher,
			Protocol:      protocol,
			ProtocolParam: "",
			Obfs:          obfs,
			ObfsParam:     "",
			Ot_enable:     0,
			Ot_domain:     "",
			Ot_path:       "",
		}, nil
	}

	moreInfo, _ := url.ParseQuery(infoPayload[1])

	// remarks
	//remarks, err := tool.Base64DecodeString(moreInfo.Get("remarks"))
	//if err != nil {
	//	remarks = ""
	//	err = nil
	//}
	//if strings.ContainsAny(remarks, "\t\r\n") {
	//	remarks = strings.ReplaceAll(remarks, "\t", "")
	//	remarks = strings.ReplaceAll(remarks, "\r", "")
	//	remarks = strings.ReplaceAll(remarks, "\n", "")
	//}
	//if strings.ContainsAny(remarks, ":/.- ") {
	//	remarks = strings.ReplaceAll(remarks, ":", "_")
	//	remarks = strings.ReplaceAll(remarks, "/", "_")
	//	remarks = strings.ReplaceAll(remarks, ".", "_")
	//	remarks = strings.ReplaceAll(remarks, "-", "_")
	//	remarks = strings.ReplaceAll(remarks, " ", "_")
	//}
	//remarks = tool.ReplaceChineseCharWith(remarks, "_")

	// protocol param
	protocolParam, err := tool.Base64DecodeString(moreInfo.Get("protoparam"))
	if err != nil {
		return nil, ErrorProtocolParamParseFail
	}
	if tool.ContainChineseChar(protocolParam) {
		protocolParam = ""
	}
	if strings.HasSuffix(protocol, "_compatible") {
		protocol = strings.ReplaceAll(protocol, "_compatible", "")
	}

	// obfs param
	obfsParam, err := tool.Base64DecodeString(moreInfo.Get("obfsparam"))
	if err != nil {
		return nil, ErrorObfsParamParseFail
	}
	if tool.ContainChineseChar(obfsParam) {
		obfsParam = ""
	}
	if strings.HasSuffix(obfs, "_compatible") {
		obfs = strings.ReplaceAll(obfs, "_compatible", "")
	}

	// ot_enable
	ot_enable, err := strconv.Atoi(moreInfo.Get("ot_enable"))
	if err != nil {
		ot_enable = 0
	}

	// ot_domain
	ot_domain, err := tool.Base64DecodeString(moreInfo.Get("ot_domain"))
	if err != nil {
		ot_domain = ""
	}

	// ot_path
	ot_path, err := tool.Base64DecodeString(moreInfo.Get("ot_path"))
	if err != nil {
		ot_path = ""
	}

	return &ShadowsocksR{
		Base: Base{
			Name:   "",
			Server: server,
			Port:   port,
			Type:   "ssr",
		},
		Password:      password,
		Cipher:        cipher,
		Protocol:      protocol,
		ProtocolParam: protocolParam,
		Obfs:          obfs,
		ObfsParam:     obfsParam,
		Ot_enable:     ot_enable,
		Ot_domain:     ot_domain,
		Ot_path:       ot_path,
	}, nil
}

var (
	ssrPlainRe = regexp.MustCompile("ssr://([A-Za-z0-9+/_-])+")
)

func GrepSSRLinkFromString(text string) []string {
	results := make([]string, 0)
	if !strings.Contains(text, "ssr://") {
		return results
	}
	texts := strings.Split(text, "ssr://")
	for _, text := range texts {
		results = append(results, ssrPlainRe.FindAllString("ssr://"+text, -1)...)
	}
	return results
}
