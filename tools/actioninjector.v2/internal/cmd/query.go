package cmd

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Sub-Command for querying IoTeX blockchain via gRPC",
	Long:  "Sub-Command for querying IoTeX blockchain via gRPC",
	RunE: func(cmd *cobra.Command, args []string) error {
		return query()
	},
}

var (
	//go:embed erc20.abi.json
	erc20JSONABI string
	erc20ABI     abi.ABI

	//go:embed api.protoset
	protosetBinary []byte

	rps          int
	concurrency  int
	total        int
	endpoint     string
	insecure     bool
	async        bool
	duration     time.Duration
	methodFilter []string

	defaultMethods = []string{
		"iotexapi.APIService.GetAccount",
		"iotexapi.APIService.EstimateActionGasConsumption",
		"iotexapi.APIService.ReadContract",
		"iotexapi.APIService.GetRawBlocks",
		"iotexapi.APIService.GetBlockMetas",
		"iotexapi.APIService.GetActions",
	}

	txHashes = []string{
		"399f37ef7b7e37acf1e3050c7d762e95de5922ca53f45f99dd02e850e69f12f6",
		"058fc8a5fbee60a199248ebb960d0d657228f108c704a67d1d2f4dd09b2aa2c0",
		"079f8d63abcfcd4f26bb9bf5dee7a5ab2d16a94ba84dce269711d5c0911ddc50",
		"3dc5245e35b70f5eaf8fc1a6d9def6758392304c6c682aaec34b515713dc40a1",
		"3d62243a0711c03b0796ed8b3e977336d9c38683b15733f9f8d78205f1447ca5",
		"1a7f372b0ff71a30669f91e73ea73f501c56e4acfca37b74f4c46e57e658b4e8",
		"bccbd4249d60f572f4a29f3b14a9abe958607899c1738c282a28922b2696eef1",
		"14283ef34e0942e7d40237b6c2b7835a3c9cadd90c227c725b6e2a94fbb78ed6",
		"f6987f36a82dce17c840f6b7247f04c4b8c2ac77f2a318b68b919f176e60e338",
		"3053fe702b37c1194e19b34a8b0394b55a9d100795f4f9a7e6b30a7faf9926d7",
		"52bd96057027c6dcf4dbabcf51d0a99d611c47b74b71101de993747e9efe5fb0",
		"dde23c3e3dcef4eff4cc4e00a62ef9a47710fa66f9fbfe13f90940a816cb9007",
		"0b5d76ed86251610136ae9698b723528a8c432cc6d5f6f8776cf10cd66ee2169",
		"8f1151f29969cfdbe85076caea565f6af69534d96ee67c940157bce0f7f8efbb",
		"7a00a55d372ee4486b99b3e6655880a28fb1f9d0ee97639d7ba79a9afe2e21c7",
		"2f5aeb6ab29bb7dfad361f3d54146b6e9a47b09f9e9f49413627af4c02f344ce",
		"98b7db2cfeceab116e88b21ed5e2dcedd31b744e2fef65f9dc21fb3b40eb0bd5",
		"8b6f005c162224257a6b7ada6e669eb088fb6e43648e47fda8f4654a532e33df",
		"754d6061d293ac2b68875a23c7f5bd5f9e91872c879295386457da2d5f1184e0",
		"6c05ec6d21a2b4b4a336fef494fc05c7615bbe0dea60663b169a6b8b8b3b5ae9",
		"6e730812576d9909cda6e1414d656a0ca93c051d34a99592208588890911fdae",
		"fdac1dc57ba583b5df27d539230a02daaeb2a260bff2a9c71f8f876ae5c28a28",
		"25c11587b606fa7c75c8ed2cc4945ace2b0621562d466b15a3820397304f93fe",
		"cc4d7594dca712fb0dc835e151d2b7cc4cfc69ed50dbb11e63757984950b5b56",
		"81ae8920ec9485da429d49155d3a24be47996c40f70081ae8d28f8c6352035e0",
		"f74601a937aac292a60f92b8874074e3ef3eee8db9e284956c23a2fb61ed2a62",
		"d128bb4b8a34cffa6a19f8a4481520034cea7659a300bc5a289c3ab03d6fdf49",
		"be0df8f7d80038d95dba20d0d2e4c5e1a3cedf176ddb8e69bd311e3213925aeb",
		"74007b42737ba569923ff4b603eddde42264960658cb205cc0fc7d54ffdadcfa",
		"67c42acb413aad1543247ace4733f08fca09bd9d0b808b384e1eb007e4980e8f",
		"ebcba771cc0aaa23089a89abeaee8cfdf500c0471951bbfd3edbbc0932436321",
		"360936db7a946f7c42519155d441e94688db6a29ccbb251f5e0d701e777c7421",
		"07750a2971cc005140c9e0a2485a27a1b1f67ec8dbfe5aee0948ddf96fde31a1",
		"649c504e6f67d1de166808bb9ecfa13d73bd7e3f214d77ddb2f25c43b6db01f1",
		"6abcaf4562a5e9f45d05428d94f38632a17b4e371d062ec575addfb410da830a",
		"e2f72388e524fba1b8c00cded8f601144fc03fe34df913f19789519436614181",
		"2f26f0737be944431b79f40e0ee5eb41d50d377a2869f9356f53a9452b49f803",
		"9bcd3864bf0487a3e31bf3414685988f5db7a46ebb7cf7a6584894b3b77407ce",
		"dc31315e9440c28389582bb6c81069b689595ced6e0f09c852131389c2bb425f",
		"00b3f8b57108e022eaef821e29cbcc76b2ed089a8c133523ec1af84bd113e300",
		"11c1f544db7770c531e0c6149e4e0a801e8b58331568cbf8563dce19ee5ad212",
		"78f42f9526229c3fe8bf96b0d410095ca54fc61c9b4d98d4787505cadefa3b28",
		"d73f2dfe14d4f294ea60918d307fa0215cb066108dff15ca329b35158f951942",
		"47ca5d51a82abbce04083b7d48d6f0894fa050a7ba643cbbf747c89e9b1e254b",
		"f487f50375fb3a696d3f45c98b93587b66752e8f202ed1c7bf8bd5374f2df836",
		"eccb0e76e976d7dbe67415d72ad873be982665f3b56dd66e3306043ce6311975",
		"3e9a9a87a8a8f126d7f1829427cb4d4f81bc7089576a5ea893306bcd0870d688",
		"7b6675a5524924baac9bcbf1398aa82c05a1385888b057c9546d956c7824350d",
		"810126a5000c6e681ff4d2ac88bd357bb364a70211bb70d9d74833f98d482d59",
		"c38d9ac75b427f5df5849785e341ea76d35aa744be2e92ce161f2569443d2aa8",
		"137179d62a02a28230e36a42eeaa0229b99fde89503c52cde28c0cd68fdf66d6",
		"08ae63b7d6ecafc42750d0c03f8dffc949ccd38aa8186e59cf2fc8bc0d604585",
		"fdca6301835c4ba1fada585b05b3dd95dc4a952722e33be59db3da05e1ddb926",
		"6d418304bddc5664ae0531ed271ee7e32602a738c9ab01d1768674f408256aca",
		"a2919e792de8cbb3fd08caa38644eec1688e166d627a5100b64c5e64bfc371f9",
		"a7093b3df1eed5fcdfcf1b2801eab7bd9cac07ba88b0f54d2fa1bb5a54cd0533",
		"3c8edd99465c76f3be71e6ad65b39912936e97ceba81117453006ec2a102c243",
		"f9894af9fa8f37fedff40af94a150f2406f3edc249e7d773e856f0d7432d108a",
		"fc165298d0b332aea6a407baefa4f4da597a965099579f576574c45fd7099e31",
		"faa30cfb48766886e4ee03a577fc2d9f2f547186413e38d3ae63b11674e91b58",
		"ec37d8ccb10cf41e84398d0099a501dad181d3a2fad4a5245d2a17c8a492eebf",
		"bc112a6e2cde67edd0ed7e62c0e718271c056d44521025a7a29d024e046671d0",
		"83d44226cff20b2aa9c05c5a68f6f897a78beedf594622c19308e3db75ba6b93",
		"4f7aa0f7324e68aed37f9e049d60a6b98ac213223cf77a8f0030bb3662a16a2f",
		"58c598e9595d9aa4ef779fd8637da994b8aae9ff2174c84e037bf8b2b6ca2249",
		"fb16b2ba3257abc00964563c92dbe9ef07eb953d0ff2df1b17641f9327784e01",
		"0c268e444fc0ccf22bbd60c8abf21b53f3e7ce1703c19f7d1aea80a47590434c",
		"ac2ea27254ff3d4b32f154398d2da99c828da06caf7225bbaf54b04b8035131c",
		"a7470510be3a8eda8c36c8f5edd56778f3208646d79964473ce389389e3e85f9",
		"1ecd5b40224a7d9ee1b948af261e214f67cb05dfaec16c6f23732317e81334ff",
		"99d51160270b312338ba7a62b796c7bfa9195e8a8cf4ef1470f7841bc74376e1",
		"c61c4a55699d193d991d413e3cc84a69f23506a725dd89544631f26efc925958",
		"780095210c0f66bc3bd89daf5465d0961f96855f5bbeaea176c50c6f034926bf",
		"4a510c3741365dc3b51c01c5c89a51c961ce7e0f15ed3a9d102b871eebdc44ef",
		"d036583164ca9030e30b5d52008e9ace982fb87a3db69205981e20ae3d3e69f0",
		"0d4bf275135340309b8bde5afcce65a5d3b7921d81d2d6c00ba144eacb7f9d32",
		"a95211502abdd443e941fe048acf6dda9456db24b01cacd36d1c3f6c10a17274",
		"9eec89506e66264b8b9b398ebe314977ddb36de63b93d478aa0b033b967c14e1",
		"3416ff552bb5ec4ca4cdb0f5d9196f5492491f28b70aabfc6a2fd633b596e749",
		"ea5c711d6d7a4f0f50e445a5a7e7e9b061dfea97eeea0f97c84af69131d23b83",
		"5ba40257025260abf63cb8d8a4321318b28608c9ca5413d45b71867e531d9beb",
		"e715f86b452458679a8d9d97dde6541427d18ecc7b4fc5a98497f3544399617c",
		"67b2486d8b9c6c29616d66093eda417c8d27aeaf489818fd7614bb0f65972bb0",
		"2a5c69e3259047ac6f2bb123af49ed9278ca7a5f7230f3ecc288bb8733017333",
		"75a50bb7031fcbd5be471a14d681907e618ad3669ced54c83e64806e46e4c487",
		"37ecb0d56000af272e6a47f8711b49a58ccd1cae7e77c7eb2ee3a4c78e1da0e1",
		"f298c9d3e23ad00d1e68d4e8392d55cc16d689609de8b66ebba23c20f5e9df92",
		"80baf53754d8d3813cdb46b46bc1060d65146eb0d963937949484b4aca5990a9",
		"217279a0c647b2707eb4ce6663e103dfe75c1376856f54a20ad7b1090cca7343",
		"3b17bf1ec6b1b17f9e691f4fb7842f809abca6d47cb65fb5b097761e08d9a2a6",
		"6057462f6ab68a876f7b728f612c04f9345d04431b3b1ce409ddbc6e2bc1053b",
		"eabf95f99956367bfbf9904bfc8b297192e479f18d16b434e67e9a7742931379",
		"dfa953d22c5a156071755a0020a47518fc2edd90d7a1e134a545c8ee23a81e91",
		"5a0f2f9ba88ca98e2603e89c610369bf4305981b28779ca1f6d270beb0527259",
		"ab9874cbf5f49e16129574b45dc6cd7d36dacf83f9a14e9363d781855720f74e",
		"68dbc70809a71785ad0b9206302a102d0c9655a76dc279cf15f85d7f719d7446",
		"3c6c2b21a02c05c069d3cc1bbce3746eb8adcd767d9c92bcc5456d89501c55cb",
		"c419cead01cc3c14fb5136baa00422812809dae4fd5e49742b7ca612e5f7c3d2",
		"38125e5981baea4a7e98715c0f0757f69743bb7415f1ec430cb25f25a55c9d14",
		"b40c8835a51f85d7454120393c0a0a32e4fb61758e756b2f0e92ea6c121ce40c",
		"c27512c9a56a87d82d0086222b265f7e17b5aae5b7cd3c362eb0caa0e74e5225",
		"019dd4730ea1deb94d47b9c24079bd6d8c8d633f64a20184fcf2094e62b73e4b",
		"967df9ea01343129b544e8a558d135e2b616d797c192fb2c52c22ca54de6c88a",
		"69ceaad042e31b2df079118e366278064a79b68fb64b92d99cfcd02d1df56103",
		"246bfedf10e6f7a7e29146e61fb7dfc50a87afc1097ed3957f7f310f7c5109c8",
		"6ce34f3a6748cc8e299131332609ff53d52ffd0ba8dfe877523c64ff986384ca",
		"65415a7619b08dda079c7dac9827e59c3f3feacb644b74e45d65587feac3fbf0",
		"99b8ccd5a66d9b331128e9e5a66946823ea744e26e9a6c0c2ab6cf5148620572",
		"e1533bf725c6412f28a80fd358b719fb5404258593d5c8ff3e284511e6d45680",
		"b4c933edef14d9d01d4bc6143d5b13ea3bb28dac9fc7473a21616b044b237cef",
		"e147a38cd2f1b3b75492b8ec8c63e8a92aee91ad37a1a8a3c9c47e51129c4b82",
		"53081a625b9e66897ca61f4bf98dbc96032f52bef7fbc40420574abae1d5c54a",
		"b209e3079bd33316cd4ab33c4bd6568f069d8210b4e5069563e876eef4f19b71",
		"d8d4bdb6a261fea0376054e14181abef91ddf822bb0265240ed2fd12aabd74c6",
		"75baf2301451361da6475e7d120320859997b05b2b56b925e7873a3f7b45e7ce",
		"90b61006277952bd468fe05085a2a23d56357031e4bc147cc64303622df413a9",
		"b5fdacc94ab17d1b9f266ea0a9743b643ab6eeaec058471df3be9564049b409b",
		"4da52b5209df11993a6795f199367a3fd19d696da5a6a61ff2ad4af4c747b53a",
		"a5a9fce38cf8694841b6273dad6dc966fa6708f4f45f762bf972d556a00003a8",
		"f4bacc6c2088f08755f2dfe254cdd73ede9e03c8598c8eb08ca6e277476c41bf",
		"bb507894ff15e3f24c51e846653fde2a69fc22400f8cc60532f83591ad75efaf",
		"87d9d6ed30dcf044df35baa1db36cadd047a54f4c9bab72bb54ce521b50007f3",
		"3c68c968d4433b0c770c7dff735097921a56bad2b4c3cb93874a3b7d81cffec5",
		"7132915ba7b5b8a323ca52502aaffb394c138bc9db37eec5eb923f628c394e94",
		"a9764088621950ced7c68cf18f24c8ea4c807bf35b6311323486a86ecaf5907e",
		"1fcca922f395d7e914785a836333073af6410a1090d3356d2166563d61277d15",
		"a5765a009b882db7901326bf6d39bd4d82d8b82b0a40764159708d5606369ab8",
		"5756f4811dfb59de821960bf0b125a9daf9ba6acc81a27d1caef4ab5d8ab9bc8",
		"9f395a787589055b50b446a5c39e8f6a1da76f6913af8d22ff6332d2b0357db5",
		"bb0cd490d520de23938a77cae0e0c8111ee0ee72372232035b72af13e04eda67",
		"8a6ebef25e76bcc17ce13f650ab9db63dbdf89613765006de07b97d74ce35055",
		"267be3dedfcb6116acddb4024ae45fae8fe4e590b6a84d8dadf5443dbec1671b",
		"8f655c0aff39f3cd1eb51aedd27bf5a0f5ca229afceddd8bcd8c8848b5143e4d",
		"38d849013be82286388737d58f32c000d2ad0437cb3153ddbc5ae5951a34be41",
		"19be7aff899f98cdd78699c9a98743bb2203396de5ae301549fd2ae849f26be0",
		"45823ee0f7f2efc6075bfac0dd1444382ed751029e014f84935c93ad3e128672",
		"efee8fde8e2bdc0743544143cc9464bef9685015060140e7faed157064779adf",
		"1e7a58ee5d1ee01b4cf3c18f1bc58e6890a98e7aa9a7b3999115aaf70a81ad89",
		"281cc214f1782948f04b3c164218b65a3d65cc6935fe54d9a05627fe2409dd03",
		"f53e4cfb2556dc50326c8afb4102c2ce15d96e2cf4356c279f65ac48ff740ff2",
		"bec2b27d1232eddec54137e70290ddd51cf7b29e03c14ca4a321ad76d20ef4ff",
		"fe77c38a3497a3a191e00d7e5d9d110821d1d6428dd10c91b16433fc34e0bc71",
		"3ccca8392648c9ab45a33ef4fc5068e8ee897df3944e0c32fe22b74e8fcc2da4",
		"2fde6df4584ade3cdef2f18357d8f80406b7cb9a8a2fde0a2e134b8634e166e6",
		"b641dec9f35cd4a0011d0b86eae27e21f7f85e91535c6623cc35a5985f68429e",
		"f46652569918f22d9ee3994db61681ffa507090d0914e84239f7211e5efce39c",
		"7a1e6e6fccf30387e4ef6f91077e835cc3a0404c57258cec28a4e2882757d746",
		"0abddd8f1785ec6887eca51aca4e1f2771a880a7f6fec8177cbdd4cad603a043",
		"721dff601f5735be9ece5feb6ae9b27b3485ccbed1a030f8f90adc03e05b7823",
		"d477507b892a99d92e8266fb46363b2cb76bf19fe3ebe8a5b5b2169899ac3c2d",
		"178629a1ccb071bb0b0fd3d7616725d82d6dc5d0c7e72e8553983155debc9fb0",
		"3f2ca9a8e4a104a7ad2841bec5d44aa840ed0d42321836e4d5489c009419dfbc",
		"b9b3f310ecda6306dd9f9b26083676973796bd3e8893ab6f87ac814100032196",
		"ef2f3971ee341606cb24f3fa430471949346068ac23bc9dc882c8942c691f9b4",
		"167ed876392624ad9d0b6046215716312f9da04eff552898fa301a1265da7935",
		"5a5e84485b8647197f2f68422ab4192c544078a59c6915535ae60de3898671c3",
		"afff07ae72b7d0c5501751fad89fb5b1f3515dd669a8396ca1b9858ab508168e",
		"aa998f48452b69e4a53be3edf1ee4673c39c483bd734a052acccbb70d75fb3fa",
		"de057c1e0ef76a805cebfaf868978cdd59ff4fc0d03c6d59b612c734b247a67c",
		"7a82d3ebb434d8f5654aeff344083f454705a73ce34f8c0e5c5c1cf605188953",
		"5c0451837ca252e5f3026cad0cd5579bbb7b216fe8650ecd28c8d72122ae1d5b",
		"4f01a2eb27e20a2c4d54a25b2bea47a77f0f0298fa41bca8c51c71f56d87f305",
		"12f12dae1a7dc6b8968972c8ee4939ae7d4c79a52c807ea7213c0c6ffe690ea9",
		"6b84ba62339bec7b368145edabfb5f454152c46dcb42a5985286eb29d3528b72",
		"9b85f5fcf52f67407844ab8587ba711f9d00ba12afab5ea15735a2a5661374da",
		"829297423d1d73fc178bccd95109306d2e021c5c16119e568c1a8538869c4e7a",
		"e2da183d4f3044b491573a5f838da7957c1770efad090f8820b6b53a321ee0b3",
		"e5d076779608b786bb29320455ea53a763d566a40c685e8d42cb1199b8d1a856",
		"74c3ceea3a2fabaf31a98e6241cfc10da128b575b1a23d38d16d2c174038cba5",
		"c13b217cdb66d8ea33b491d57dc08ffc5ba9257dfced3addee70dadf1125dd9b",
		"87e2791fd7e035aff01368696efbf63ed4acf96ff7b3f422c9666127c0abc9d0",
		"e1a7c2565a2f610d8bee013902b4b494c702674d5388c87d1adc391372de6235",
		"e86a2af6c87498c4234e7161858b3ca465c42bdba3466f35f99816b21c7ff011",
		"6e6d2e905d861066c21b1b9c0dad867b38458742d80dda69a301f1ea7f1ab4c7",
		"5062618b41cbf14220c4cf55b39d65e3845c0e87d25143d315b52f0a7e68e389",
		"e06d954954f54cc7cfb6930e4561c5c08118a51979f3e2291469aae30e6fd365",
		"90c270e3547bfe5f14725ee47391ace415f5633b42bbd9ac2f4e820ce91c86b7",
		"886df9585dcdeb9577c1900f8ebab3b707c76a20b888b379caa4a5a3471bdd0c",
		"886f58fc3176b7860628afe627ff55f9621828e0e1c3c6ce25ac6792ef5109ac",
		"7fb53d76fb678f7c655ee8002a9afae308e8f74267676705f1b7118d7aeac56c",
		"699c06a9cfca4d6dae1b13dfd42439a1d0301d5bb499ee81776b0cf0f7e5eff0",
		"493d7071d621e5f9b68a0aac2f5f1853d9c4041eeb2e45512bd980341e807fd4",
		"070d18d681a66e14fd0314fac4c5f15befb78ed53f0a7687a65f0847cb7d61fb",
		"e7f9fac3930a727cc1083d610999d47ad2bac80992d1059b36a084d8e81c51af",
		"c27942db5906c54919fca43530a733d7e823be169df1caf92b0ffcaf9012d96a",
		"748c19632fd79aa2d713bdda68f69dfd4984f0ebbbf8ca7a23ac42da1e5c4d30",
		"46b1903259a1cd09fa7dee9716b121b98e9581d12498e7a508e6830139232c70",
		"70575eff71d6e43592e911fd6ecb7e9eadbf80357e8f32ce678940ef365143cc",
		"af3a8c2e69724bc6dc949b448eaac29690f0108f7ab7f277d5ccde3901031e8c",
		"feb1692df4fcc56be0f51cea87fb26e14bb19aee0c0fcc45241f108e20cc9595",
		"4813050dc496bf21f7ea0bdfd3bac12d47acc38b168e242587597ba3b2b027cf",
		"15678274fc66034b1ab7eb206c940c73092f0a9319d4ecdaea54d0ea0ea01360",
		"074c779971cff729624fd3fae41428fd3b5b3a8894cdd7f5cf53f44b7a840bc3",
		"10f851775739aca16b7620484df2e9e4af7203dff32393c98566b42319f4fb86",
		"448723d7161905a5f1532aabda1c46306f32f3450600e7da75a8258c721c6c10",
		"9442a0b55b1320b546905cf02b216adf6eaa189c4c819840ea5b8954182be13d",
		"6adf1a4932bdad6ab61e667ca0b24bbc6b37cf4d7af93cb181ccc0a9c9a423a7",
		"18ab299910a05b0375ce9e7e9e90582939a344df805fcc7657bdec681d6004e2",
		"c14c04fcd00e016bd445a60db5b26f00be746609b4ef857627321ac59800bf13",
		"580656568f69fee3d08513f1afa443dabc8539c2782944f9a16955ecd34418a1",
		"e3ac0092e8d61a2ef5b1c03fc174f4ffcf60e8b430d6e6ae35cd89f0aa5d0461",
		"37d1f5e26055632e5017fe044089c322210caf57a9a02ce46a119a8158a99f30",
		"b46d79a366d72ae46271b7e6254299a16e71e8ec98d0589d97f709a387006bb1",
		"277eab03a037ce08322f501cbda875760f28dff81e179a1508f41db94777bcdc",
		"ad7ce35db10e2bf1ae1463957122999222668ab88261f0c3ab851411f968af29",
		"f2ff721b10a90430f95d9a654aa70ec32de73511ee58e258d63a75c569057513",
		"96d4b0717ba6dc2167ceac6e23b64ddca46bb10b830c4aa46182c47fbd824fdf",
		"9a17698f232c833c4f78141f563419d032352c6c777be2660b9942d1bc786378",
		"cd1fdb89a061786cfda583172b7fc8f3c8f44771c17e8bab332303daeecdfc55",
		"a56610b146a3753abc3667b3158fa60d2c157c2cc685a3250c8593db50812a53",
		"80730cdd12823568dccc905abfc977cef9c4a4cd4111ed16d44bc2d4fa01effd",
		"205a4a1652cf86900b2bc3aae155ebb83af9abfe2659fa8e047c51418002b76f",
		"49aefc35fe5b3fb134e922b31a9fcafdeea56d893bd860ce6eaec934027ca7fa",
		"10be0f574e4c6f8e5398dab3c53581260ab1dbb8b121aa36b04958bd466569ad",
		"60e140a7fd2ebfa11013c4de026493ef095b37f2eac00594d788e50f8bb88278",
		"7e9cfb8715815356b4ab0ec4e9dff5505ed145f7322b5f304e29623706189b1c",
		"57a494766a045dbb87e7081154b7490b7b16b40beb6a5f2a51bf770b54e59507",
		"29ef9877cd147a078043076e47f46c104cd779b48729b4f91520c0a096920847",
		"16a68859510f17f15dae4a44f80918f866580a39c5eb2b2c37f252d5d5b1f79e",
		"7352e8b79e25840a4f8f5526cbfea8f1f0eb15a0514a52451c1f323d231b5e5d",
		"3ecb304ec5b2302bcbe6fd45c494f322680bbec7b12f4ccddd20e74df7e4a449",
		"9165c9ec581ecbe54b68534e6407815f775e3cf3d14efe5f1996168b2b2fe558",
		"90cec03ee1581e7c6b827bc5ea7d4c645235367d711b57bc7eb0d27729ceed30",
		"3b0ca31b99bb47241b9ea1d91faa6b46171ead862b05e729e8b4f08858622a11",
		"fc1d652387003afc13f6a8b7f7ea161da5675e4634a027e1427e06df3d350310",
		"da0af278f1d52a4de030f8c5b4e050edc27577c1be01edd1cd7293dc469420bc",
		"56bd9681230c2f79170c098d2f10db06f911596c845ba75a2ece984ed912d00b",
		"940c4c5158d97f2c258838ee39209cdb2ac39274cbafc1594b311557ce89a39d",
		"1e5f0d3976a088c31468ffae4b349d2a7fa7014ea079f0ede3b2b501a09ae50f",
		"6ef7d0caf00801a9352805302950e709665f1f3d4201d4a31eade5893915e54f",
		"9471781cedc0e4b13e02dd9e78fb62f52e3cf83c3e35d480ef2e1c59f5789b21",
		"c72f42a482240aead6ee08002e622e8bb966b1d013b6a507792b327058eaaf7c",
		"97861eaee49480a1d258ef0ba9cf583fc25f20d1dac30ef3233e9777a51e6a05",
		"5ae03aa265ae7a2b6cdbaa1c97894df0b101f691a902c24d091bea1786c766de",
		"eb8bcf7166f448793281605addab4e399e27f4beb18d832c81ad1382241df67e",
		"c80ea457c96594e6a077b9c4ddebed2389baaf2549e8f26548c14cd79fe65ea1",
		"5af24073b33178a0ee030401683b42761194d03a8031857b3a3cebd99569a9e7",
		"ad27c7f94b5e30ddc6af40442847a3a23a390afefc0c8f4bf7fb66eeb41b3bc0",
		"51a6a0790f557eb096ca3ff137bb6873e6e864062506af9fade5c992cc388026",
		"21b4eea90984fbd3c56adc5d43b900cf9d3a0c1f3fdd860f21562b457dbc9faf",
		"a08384b27120daac1e2f89c07e876538aebcce3f28151c4da0a060a2e9fae050",
	}

	tip       atomic.Uint64
	actHashes atomic.Value
	api       iotexapi.APIServiceClient
)

func init() {
	var err error
	erc20ABI, err = abi.JSON(strings.NewReader(erc20JSONABI))
	if err != nil {
		panic(err)
	}

	queryCmd.PersistentFlags().IntVarP(&rps, "rps", "", 100, "Number of requests per second")
	queryCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "", 10, "Number of requests to run concurrently")
	// queryCmd.PersistentFlags().IntVarP(&total, "total", "", 100, "Number of requests to run")
	queryCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "", "api.testnet.iotex.one:443", "gRPC endpoint to query")
	queryCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "", false, "Use insecure connection")
	queryCmd.PersistentFlags().BoolVarP(&async, "async", "", false, "Use async mode")
	queryCmd.PersistentFlags().DurationVarP(&duration, "duration", "", 0, "Duration of test")
	queryCmd.PersistentFlags().StringSliceVarP(&methodFilter, "methods", "", defaultMethods, "Methods to query")
	rootCmd.AddCommand(queryCmd)
}

func query() error {
	var conn *grpc.ClientConn
	var err error
	if insecure {
		conn, err = grpc.NewClient(rawInjectCfg.serverAddr, grpc.WithTransportCredentials(grpcInsecure.NewCredentials()))
	} else {
		conn, err = grpc.NewClient(rawInjectCfg.serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	if err != nil {
		log.L().Fatal("Failed to create gRPC client", zap.Error(err))
	}
	api = iotexapi.NewAPIServiceClient(conn)
	syncTip()
	log.L().Info("sync tip", zap.Uint64("tip", tip.Load()))
	go func() {
		for {
			syncTip()
			time.Sleep(5 * time.Second)
		}
	}()

	methods := queryMethods()
	reports := make([]*runner.Report, len(methods))
	g := errgroup.Group{}
	conMethod := concurrency / len(methods)
	rpcMethod := rps / len(methods)
	for i := range methods {
		id := i
		g.Go(func() error {
			report, err := runner.Run(
				methods[id],
				endpoint,
				runner.WithProtosetBinary(protosetBinary),
				runner.WithConcurrency(uint(conMethod)),
				runner.WithAsync(async),
				runner.WithRPS(uint(rpcMethod)),
				runner.WithRunDuration(duration),
				runner.WithDataProvider(provider),
				runner.WithInsecure(insecure),
			)
			if err != nil {
				return err
			}
			reports[id] = report
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for i, report := range reports {
		printer := printer.ReportPrinter{
			Out:    os.Stdout,
			Report: report,
		}
		printer.Out.Write([]byte("--------------------\n"))
		printer.Out.Write([]byte(fmt.Sprintf("RPC Method: %s\n", methods[i])))
		printer.Print("summary")
	}
	return nil
}

func queryMethods() []string {
	methods := make([]string, 0)
	filter := func(s string) bool {
		for _, f := range methodFilter {
			if strings.Contains(s, f) {
				return true
			}
		}
		return false
	}
	for _, m := range defaultMethods {
		if filter(m) {
			methods = append(methods, m)
		}
	}
	return methods
}

func provider(call *runner.CallData) ([]*dynamic.Message, error) {
	var protoMsg protoiface.MessageV1
	switch call.MethodName {
	case "ReadContract":
		data, err := erc20ABI.Pack("balanceOf", common.BytesToAddress(identityset.Address(rand.Intn(30)).Bytes()))
		if err != nil {
			return nil, err
		}
		execution := action.NewExecution("io1qfvgvmk6lpxkpqwlzanqx4atyzs86ryqjnfuad", big.NewInt(0), data)
		protoMsg = &iotexapi.ReadContractRequest{
			Execution:     execution.Proto(),
			CallerAddress: identityset.Address(rand.Intn(30)).String(),
			GasLimit:      0,
			GasPrice:      "0",
		}
	case "EstimateActionGasConsumption":
		data, err := erc20ABI.Pack("balanceOf", common.BytesToAddress(identityset.Address(rand.Intn(30)).Bytes()))
		if err != nil {
			return nil, err
		}
		execution := action.NewExecution("io1qfvgvmk6lpxkpqwlzanqx4atyzs86ryqjnfuad", big.NewInt(0), data)
		protoMsg = &iotexapi.EstimateActionGasConsumptionRequest{
			Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
				Execution: execution.Proto(),
			},
			CallerAddress: identityset.Address(rand.Intn(30)).String(),
			GasPrice:      "0",
		}
	case "GetAccount":
		protoMsg = &iotexapi.GetAccountRequest{
			Address: identityset.Address(rand.Intn(30)).String(),
		}
	case "GetRawBlocks":
		protoMsg = &iotexapi.GetRawBlocksRequest{
			StartHeight:         tip.Load() - uint64(rand.Intn(64)),
			Count:               3,
			WithReceipts:        true,
			WithTransactionLogs: true,
		}
	case "GetBlockMetas":
		protoMsg = &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: tip.Load() - uint64(rand.Intn(64)),
					Count: 10,
				},
			},
		}
	case "GetActions":
		protoMsg = &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash: txHashes[rand.Intn(len(txHashes))],
				},
			},
		}
	default:
		return nil, errors.Errorf("not supported method %s", call.MethodName)
	}

	dynamicMsg, err := dynamic.AsDynamicMessage(protoMsg)
	if err != nil {
		return nil, err
	}
	return []*dynamic.Message{dynamicMsg}, nil
}

func syncTip() {
	resp, err := api.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
	if err != nil {
		log.L().Error("Failed to get chain meta", zap.Error(err))
	}
	tip.Store(resp.ChainMeta.Height)
}
