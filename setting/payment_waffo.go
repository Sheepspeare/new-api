package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

var WaffoEnabled = false
var WaffoApiKey = ""
var WaffoPrivateKey = ""
var WaffoPublicCert = ""
var WaffoSandboxPublicCert = ""
var WaffoSandboxApiKey = ""
var WaffoSandboxPrivateKey = ""
var WaffoSandbox = false
var WaffoMerchantId = ""
var WaffoCurrency = "USD"
var WaffoUnitPrice = 1.0
var WaffoMinTopUp = 1
var WaffoNotifyUrl = ""
var WaffoReturnUrl = ""
var WaffoPayMethods = ""

var WaffoPancakeEnabled = false
var WaffoPancakeSandbox = false
var WaffoPancakeCurrency = "USD"
var WaffoPancakeMerchantID = ""
var WaffoPancakeStoreID = ""
var WaffoPancakeProductID = ""
var WaffoPancakePrivateKey = ""
var WaffoPancakeReturnURL = ""
var WaffoPancakeWebhookPublicKey = ""
var WaffoPancakeWebhookTestKey = ""
var WaffoPancakeUnitPrice = 1.0
var WaffoPancakeMinTopUp = 1

func GetWaffoPayMethods() []constant.WaffoPayMethod {
	methods := make([]constant.WaffoPayMethod, len(constant.DefaultWaffoPayMethods))
	copy(methods, constant.DefaultWaffoPayMethods)
	if strings.TrimSpace(WaffoPayMethods) == "" {
		return methods
	}

	var parsed []constant.WaffoPayMethod
	if err := common.UnmarshalJsonStr(WaffoPayMethods, &parsed); err != nil || len(parsed) == 0 {
		return methods
	}
	return parsed
}
