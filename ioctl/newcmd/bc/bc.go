// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bc

import "github.com/iotexproject/iotex-core/ioctl/config"

// Multi-language support
var (
	bcCmdShorts = map[config.Language]string{
		config.English: "Deal with block chain of IoTeX blockchain",
		config.Chinese: "处理IoTeX区块链上的区块",
	}
	flagEndpointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点",
	}
	flagInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全的连接",
	}
)
