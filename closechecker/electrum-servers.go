package closechecker

type electrumServer struct {
	Server string
	TLS    string
	TCP    string
}

var electrumServers = []electrumServer{
	{
		Server: "104.248.139.211",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "128.0.190.26",
		TLS:    "50002",
	},
	{
		Server: "142.93.6.38",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "157.245.172.236",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "159.65.53.177",
		TCP:    "50001",
	},
	{
		Server: "167.172.42.31",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "2AZZARITA.hopto.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "2electrumx.hopto.me",
		TLS:    "56022",
		TCP:    "56021",
	},
	{
		Server: "2ex.digitaleveryware.com",
		TLS:    "50002",
	},
	{
		Server: "37.205.9.165",
		TLS:    "50002",
	},
	{
		Server: "68.183.188.105",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "73.92.198.54",
		TLS:    "50002",
	},
	{
		Server: "89.248.168.53",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "E-X.not.fyi",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "VPS.hsmiths.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "alviss.coinjoined.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "assuredly.not.fyi",
		TLS:    "50002",
	},
	{
		Server: "bitcoin.aranguren.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "bitcoin.lukechilds.co",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "bitcoins.sk",
		TLS:    "56002",
		TCP:    "56001",
	},
	{
		Server: "blackie.c3-soft.com",
		TLS:    "57002",
		TCP:    "57001",
	},
	{
		Server: "blkhub.net",
		TLS:    "50002",
	},
	{
		Server: "blockstream.info",
		TLS:    "700",
		TCP:    "110",
	},
	{
		Server: "btc.electroncash.dk",
		TLS:    "60002",
		TCP:    "60001",
	},
	{
		Server: "btc.litepay.ch",
		TLS:    "50002",
	},
	{
		Server: "btc.ocf.sh",
		TLS:    "50002",
	},
	{
		Server: "btce.iiiiiii.biz",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "de.poiuty.com",
		TLS:    "50002",
		TCP:    "50004",
	},
	{
		Server: "e.keff.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "e2.keff.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "eai.coincited.net",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "ecdsa.net",
		TLS:    "110",
		TCP:    "50001",
	},
	{
		Server: "electrum.bitaroo.net",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrum.blockstream.info",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrum.dcn.io",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrum.emzy.de",
		TLS:    "50002",
	},
	{
		Server: "electrum.hodlister.co",
		TLS:    "50002",
	},
	{
		Server: "electrum.hsmiths.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrum.jochen-hoenicke.de",
		TLS:    "50006",
		TCP:    "50099",
	},
	{
		Server: "electrum.pabu.io",
		TLS:    "50002",
	},
	{
		Server: "electrum.qtornado.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrum3.hodlister.co",
		TLS:    "50002",
	},
	{
		Server: "electrum5.hodlister.co",
		TLS:    "50002",
	},
	{
		Server: "electrumx.alexridevski.net",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrumx.erbium.eu",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "electrumx.schulzemic.net",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "elx.bitske.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "ex.btcmp.com",
		TLS:    "50002",
	},
	{
		Server: "ex03.axalgo.com",
		TLS:    "50002",
	},
	{
		Server: "exs.dyshek.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "fortress.qtornado.com",
		TLS:    "443",
	},
	{
		Server: "fulcrum.grey.pw",
		TLS:    "51002",
		TCP:    "51001",
	},
	{
		Server: "gall.pro",
		TLS:    "50002",
	},
	{
		Server: "guichet.centure.cc",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "hodlers.beer",
		TLS:    "50002",
	},
	{
		Server: "horsey.cryptocowboys.net",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "kareoke.qoppa.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "lavahost.org",
		TLS:    "50002",
	},
	{
		Server: "node.degga.net",
		TLS:    "50002",
	},
	{
		Server: "node1.btccuracao.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "skbxmit.coinjoined.com",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "smmalis37.ddns.net",
		TLS:    "50002",
	},
	{
		Server: "stavver.dyshek.org",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "tardis.bauerj.eu",
		TLS:    "50002",
		TCP:    "50001",
	},
	{
		Server: "vmd71287.contaboserver.net",
		TLS:    "50002",
	},
	{
		Server: "vmd84592.contaboserver.net",
		TLS:    "50002",
	},
	{
		Server: "xtrum.com",
		TLS:    "50002",
		TCP:    "50001",
	},
}

var electrumTestnetServers = []electrumServer{
	{
		Server: "blackie.c3-soft.com",
		TLS:    "57006",
		TCP:    "57005",
	},
	{
		Server: "blockstream.info",
		TLS:    "993",
		TCP:    "143",
	},
	{
		Server: "electrum.blockstream.info",
		TLS:    "60002",
		TCP:    "60001",
	},
	{
		Server: "testnet.aranguren.org",
		TLS:    "51002",
		TCP:    "51001",
	},
	{
		Server: "testnet.hsmiths.com",
		TLS:    "53012",
	},
	{
		Server: "testnet.qtornado.com",
		TLS:    "51002",
		TCP:    "51001",
	},
	{
		Server: "tn.not.fyi",
		TLS:    "55002",
		TCP:    "55001",
	},
}
