package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeDelegateCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return(
		"mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()

	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(
		apiServiceClient, nil).AnyTimes()

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{Epoch: &iotextypes.EpochData{Num: 7000}}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).AnyTimes()

	var testBlockProducersInfo = []*iotexapi.BlockProducerInfo{
		{Address: "io1kr8c6krd7dhxaaqwdkr6erqgu4z0scug3drgja", Votes: "109510794521770016955545668", Active: true, Production: 30},
		{Address: "io1456knehzn9qup8unxlf4q06empz8lqxtp6v5vh", Votes: "102571143552077397264727139", Active: true, Production: 30},
		{Address: "io1ddjluttkzljqfgdtcz9eu3r3s832umy7ylx0ts", Votes: "98090078890392421668851404", Active: true, Production: 30},
		{Address: "io1hgxksz37qtqq9n5n6lkkc9qhaajdklhvkh5969", Votes: "88872577987265387615824262", Active: true, Production: 30},
		{Address: "io14u5d66rt465ykm7t2847qllj0reml27q30kr75", Votes: "84052361753868321848541599", Active: true, Production: 30},
		{Address: "io108h7sa5sap44e244hz649zyk5y4rqzsvnpzxh5", Votes: "83017661686050993422089061", Active: true, Production: 30},
		{Address: "io13q2am9nedrd3n746lsj6qan4pymcpgm94vvx2c", Votes: "81497052527306018062463878", Active: false, Production: 0},
		{Address: "io1gfq9el2gnguus64ex3hu8ajd6e4yzk3f9cz5vx", Votes: "73054892390468670695397385", Active: true, Production: 30},
		{Address: "io1qnec80ark9shjc6uzk45dhm8s50dpc27sjuver", Votes: "72800827559311526331738530", Active: false, Production: 0},
		{Address: "io18tnk59jqdjgpdzz4hq4dl0dwwjkv7gg20fygqn", Votes: "63843185329531983507188064", Active: false, Production: 0},
		{Address: "io1qqaswtu7rcevahucpfxc0zxx088pylwtxnkrrl", Votes: "63494040672622657908022915", Active: true, Production: 30},
		{Address: "io1puxw08ze2jnv5x4943ly273tkyth56p9k6ssv9", Votes: "61309224009653017648938445", Active: true, Production: 30},
		{Address: "io1e5vwdmpkf2hpu2266hx9syu0ursqgqwv0kzrxm", Votes: "58398677997892390932401775", Active: true, Production: 30},
		{Address: "io1et7zkzc76m9twa4gn5xht3urt9mwj05qvdtj66", Votes: "57884530935762751497295107", Active: false, Production: 0},
		{Address: "io1fra0fx6akacny9asewt7vyvggqq4rtq3eznccr", Votes: "57699934366242359064327183", Active: false, Production: 0},
		{Address: "io1mcyk4aqmwgn7zjx0n9ggdtpm9x6y9pkdze3u39", Votes: "56936017082584646690083786", Active: true, Production: 30},
		{Address: "io1tf7tu2xt6mpk8s70ahugsx20jqgu9eg6v48qlk", Votes: "54356930593139259174591454", Active: true, Production: 30},
		{Address: "io10reczcaelglh5xmkay65h9vw3e5dp82e8vw0rz", Votes: "54142015787191991384650558", Active: true, Production: 30},
		{Address: "io195mh6ftwz5vnagw984fj4hj8awty3ue2gh457f", Votes: "53438501418406316391702941", Active: false, Production: 0},
		{Address: "io1vl0wnuwtjv46deah8qny82g5d9rr45cl0fcsg3", Votes: "50439414145173805775719477", Active: true, Production: 30},
		{Address: "io12fa96hxh9ata23gwp5pfxztzaawvwl2sk5xmdg", Votes: "47616018438106266407182199", Active: true, Production: 30},
		{Address: "io1tfke5nfwryte6nultpmqefadgm0dsahm2gm63k", Votes: "47547651449309094613905900", Active: true, Production: 30},
		{Address: "io1cw8x7ruxeqqcklmlxc5yxqfu2txd2hgv8rt6j7", Votes: "45296383525563818835234318", Active: false, Production: 0},
		{Address: "io1zhefl8l5e9sw93kf3f7jz7an9zqqnvkvyxd4f0", Votes: "44753889793475843962376641", Active: true, Production: 30},
		{Address: "io13672hgsdr7d8nu5jndyageygel3r5nwyzqwlhl", Votes: "43351538438466056259112950", Active: false, Production: 0},
		{Address: "io1lcpdct3sr7lg53x23aylungmmdrn5jq0vlnclu", Votes: "42277928969095512626332913", Active: false, Production: 0},
		{Address: "io1z4sxtefurklkyrfmmdtjmw4h8csnxlv9747hyd", Votes: "41525283055065104346674271", Active: true, Production: 30},
		{Address: "io1ckvhh2nym5ejehklmn3ql6c52f6nfdnguspj5n", Votes: "41346686489279940040900234", Active: true, Production: 30},
		{Address: "io13xdhg9du56khumz3sg6a3ma5q5kjx7vdlx48x8", Votes: "40446200570810948673568854", Active: true, Production: 30},
		{Address: "io1ztqalgh0zl9309a48k7wjwyump6agq24cf4zdq", Votes: "39737835497414451631412012", Active: false, Production: 0},
		{Address: "io1n3llm0zzlles6pvpzugzajyzyjehztztxf2rff", Votes: "39619838039171064163081805", Active: true, Production: 32},
		{Address: "io1ug4qkvsyapznevum0sdj9tl37qdhcwz9cxy6ks", Votes: "38699057276054350901849107", Active: true, Production: 28},
		{Address: "io1zy9q4myp3jezxdsv82z3f865ruamhqm7a6mggh", Votes: "38006759895576284703944491", Active: true, Production: 30},
		{Address: "io1y5yafr6y48ck234hudvnslszutma9vhdy4l2au", Votes: "37923375250314530385988502", Active: false, Production: 0},
		{Address: "io1u4pev9u5as7ujpk29wpsrc95m8kuhec3pmuk72", Votes: "37259902890070294856582714", Active: true, Production: 30},
		{Address: "io1hczkhdcpf2a7dxhydspxqp0ycmu9ejyud6v43d", Votes: "36504518325780350328469428", Active: false, Production: 0},
	}
	epochMetaResponse := &iotexapi.GetEpochMetaResponse{EpochData: &iotextypes.EpochData{Num: 7000, Height: 3223081}, TotalBlocks: 720, BlockProducersInfo: testBlockProducersInfo}
	apiServiceClient.EXPECT().GetEpochMeta(gomock.Any(), gomock.Any()).Return(epochMetaResponse, nil).AnyTimes()

	probationList := &iotexapi.ReadStateResponse{}
	apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(probationList, nil).AnyTimes()

	t.Run("get zero delegate epoch", func(t *testing.T) {
		cmd := NewNodeDelegateCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "io1kr8c6krd7dhxaaqwdkr6erqgu4z0scug3drgja")
		require.Contains(result, "109510794.521770016955545668")

	})
}
