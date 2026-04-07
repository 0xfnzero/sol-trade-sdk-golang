package swqos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	soltradesdk "github.com/your-org/sol-trade-sdk-go/pkg"
	"github.com/gagliardetto/solana-go"
)

// ===== Type aliases (avoid duplicate definitions with pkg/types.go) =====

type SwqosType = soltradesdk.SwqosType
type SwqosRegion = soltradesdk.SwqosRegion
type TradeType = soltradesdk.TradeType

const (
	SwqosTypeJito        = soltradesdk.SwqosTypeJito
	SwqosTypeNextBlock   = soltradesdk.SwqosTypeNextBlock
	SwqosTypeZeroSlot    = soltradesdk.SwqosTypeZeroSlot
	SwqosTypeTemporal    = soltradesdk.SwqosTypeTemporal
	SwqosTypeBloxroute   = soltradesdk.SwqosTypeBloxroute
	SwqosTypeNode1       = soltradesdk.SwqosTypeNode1
	SwqosTypeFlashBlock  = soltradesdk.SwqosTypeFlashBlock
	SwqosTypeBlockRazor  = soltradesdk.SwqosTypeBlockRazor
	SwqosTypeAstralane   = soltradesdk.SwqosTypeAstralane
	SwqosTypeStellium    = soltradesdk.SwqosTypeStellium
	SwqosTypeLightspeed  = soltradesdk.SwqosTypeLightspeed
	SwqosTypeSoyas       = soltradesdk.SwqosTypeSoyas
	SwqosTypeSpeedlanding = soltradesdk.SwqosTypeSpeedlanding
	SwqosTypeHelius      = soltradesdk.SwqosTypeHelius
	SwqosTypeDefault     = soltradesdk.SwqosTypeDefault

	SwqosRegionNewYork    = soltradesdk.SwqosRegionNewYork
	SwqosRegionFrankfurt  = soltradesdk.SwqosRegionFrankfurt
	SwqosRegionAmsterdam  = soltradesdk.SwqosRegionAmsterdam
	SwqosRegionSLC        = soltradesdk.SwqosRegionSLC
	SwqosRegionTokyo      = soltradesdk.SwqosRegionTokyo
	SwqosRegionLondon     = soltradesdk.SwqosRegionLondon
	SwqosRegionLosAngeles = soltradesdk.SwqosRegionLosAngeles
	SwqosRegionDefault    = soltradesdk.SwqosRegionDefault

	TradeTypeBuy  = soltradesdk.TradeTypeBuy
	TradeTypeSell = soltradesdk.TradeTypeSell
)

// ===== Constants =====

// Minimum tips in SOL for each provider
const (
	MinTipJito         = 0.00001
	MinTipNextBlock    = 0.001
	MinTipZeroSlot     = 0.0001
	MinTipTemporal     = 0.0001
	MinTipBloxroute    = 0.0001
	MinTipNode1        = 0.0001
	MinTipFlashBlock   = 0.0001
	MinTipBlockRazor   = 0.0001
	MinTipAstralane    = 0.00001
	MinTipStellium     = 0.0001
	MinTipLightspeed   = 0.0001
	MinTipSoyas        = 0.001
	MinTipSpeedlanding = 0.001
	MinTipHelius       = 0.000005 // SWQOS-only mode
	MinTipDefault      = 0.0
)

// ===== Tip Accounts =====

var jitoTipAccounts = []string{
	"96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5",
	"HFqU5x63VTqvQss8hp11i4wVV8bD44PvwucfZ2bU7gRe",
	"Cw8CFyM9FkoMi7K7Crf6HNQqf4uEMzpKw6QNghXLvLkY",
	"ADaUMid9yfUytqMBgopwjb2DTLSokTSzL1zt6iGPaS49",
	"DfXygSm4jCyNCybVYYK6DwvWqjKee8pbDmJGcLWNDXjh",
	"ADuUkR4vqLUMWXxW9gh6D6L8pMSawimctcNZ5pGwDcEt",
	"DttWaMuVvTiduZRnguLF7jNxTgiMBZ1hyAumKUiL2KRL",
	"3AVi9Tg9Uo68tJfuvoKvqKNWKkC5wPdSSdeBnizKZ6jT",
}

var nextBlockTipAccounts = []string{
	"NextbLoCkVtMGcV47JzewQdvBpLqT9TxQFozQkN98pE",
	"NexTbLoCkWykbLuB1NkjXgFWkX9oAtcoagQegygXXA2",
	"NeXTBLoCKs9F1y5PJS9CKrFNNLU1keHW71rfh7KgA1X",
	"NexTBLockJYZ7QD7p2byrUa6df8ndV2WSd8GkbWqfbb",
	"neXtBLock1LeC67jYd1QdAa32kbVeubsfPNTJC1V5At",
	"nEXTBLockYgngeRmRrjDV31mGSekVPqZoMGhQEZtPVG",
	"NEXTbLoCkB51HpLBLojQfpyVAMorm3zzKg7w9NFdqid",
	"nextBLoCkPMgmG8ZgJtABeScP35qLa2AMCNKntAP7Xc",
}

var zeroSlotTipAccounts = []string{
	"Eb2KpSC8uMt9GmzyAEm5Eb1AAAgTjRaXWFjKyFXHZxF3",
	"FCjUJZ1qozm1e8romw216qyfQMaaWKxWsuySnumVCCNe",
	"ENxTEjSQ1YabmUpXAdCgevnHQ9MHdLv8tzFiuiYJqa13",
	"6rYLG55Q9RpsPGvqdPNJs4z5WTxJVatMB8zV3WJhs5EK",
	"Cix2bHfqPcKcM233mzxbLk14kSggUUiz2A87fJtGivXr",
}

var temporalTipAccounts = []string{
	"TEMPaMeCRFAS9EKF53Jd6KpHxgL47uWLcpFArU1Fanq",
	"noz3jAjPiHuBPqiSPkkugaJDkJscPuRhYnSpbi8UvC4",
	"noz3str9KXfpKknefHji8L1mPgimezaiUyCHYMDv1GE",
	"noz6uoYCDijhu1V7cutCpwxNiSovEwLdRHPwmgCGDNo",
	"noz9EPNcT7WH6Sou3sr3GGjHQYVkN3DNirpbvDkv9YJ",
	"nozc5yT15LazbLTFVZzoNZCwjh3yUtW86LoUyqsBu4L",
	"nozFrhfnNGoyqwVuwPAW4aaGqempx4PU6g6D9CJMv7Z",
	"nozievPk7HyK1Rqy1MPJwVQ7qQg2QoJGyP71oeDwbsu",
	"noznbgwYnBLDHu8wcQVCEw6kDrXkPdKkydGJGNXGvL7",
	"nozNVWs5N8mgzuD3qigrCG2UoKxZttxzZ85pvAQVrbP",
	"nozpEGbwx4BcGp6pvEdAh1JoC2CQGZdU6HbNP1v2p6P",
	"nozrhjhkCr3zXT3BiT4WCodYCUFeQvcdUkM7MqhKqge",
	"nozrwQtWhEdrA6W8dkbt9gnUaMs52PdAv5byipnadq3",
	"nozUacTVWub3cL4mJmGCYjKZTnE9RbdY5AP46iQgbPJ",
	"nozWCyTPppJjRuw2fpzDhhWbW355fzosWSzrrMYB1Qk",
	"nozWNju6dY353eMkMqURqwQEoM3SFgEKC6psLCSfUne",
	"nozxNBgWohjR75vdspfxR5H9ceC7XXH99xpxhVGt3Bb",
}

var bloxrouteTipAccounts = []string{
	"HWEoBxYs7ssKuudEjzjmpfJVX7Dvi7wescFsVx2L5yoY",
	"95cfoy472fcQHaw4tPGBTKpn6ZQnfEPfBgDQx6gcRmRg",
	"3UQUKjhMKaY2S6bjcQD6yHB7utcZt5bfarRCmctpRtUd",
	"FogxVNs6Mm2w9rnGL1vkARSwJxvLE8mujTv3LK8RnUhF",
}

var node1TipAccounts = []string{
	"node1PqAa3BWWzUnTHVbw8NJHC874zn9ngAkXjgWEej",
	"node1UzzTxAAeBTpfZkQPJXBAqixsbdth11ba1NXLBG",
	"node1Qm1bV4fwYnCurP8otJ9s5yrkPq7SPZ5uhj3Tsv",
	"node1PUber6SFmSQgvf2ECmXsHP5o3boRSGhvJyPMX1",
	"node1AyMbeqiVN6eoQzEAwCA6Pk826hrdqdAHR7cdJ3",
	"node1YtWCoTwwVYTFLfS19zquRQzYX332hs1HEuRBjC",
}

var flashBlockTipAccounts = []string{
	"FLaShB3iXXTWE1vu9wQsChUKq3HFtpMAhb8kAh1pf1wi",
	"FLashhsorBmM9dLpuq6qATawcpqk1Y2aqaZfkd48iT3W",
	"FLaSHJNm5dWYzEgnHJWWJP5ccu128Mu61NJLxUf7mUXU",
	"FLaSHR4Vv7sttd6TyDF4yR1bJyAxRwWKbohDytEMu3wL",
	"FLASHRzANfcAKDuQ3RXv9hbkBy4WVEKDzoAgxJ56DiE4",
	"FLasHstqx11M8W56zrSEqkCyhMCCpr6ze6Mjdvqope5s",
	"FLAShWTjcweNT4NSotpjpxAkwxUr2we3eXQGhpTVzRwy",
	"FLasHXTqrbNvpWFB6grN47HGZfK6pze9HLNTgbukfPSk",
	"FLAshyAyBcKb39KPxSzXcepiS8iDYUhDGwJcJDPX4g2B",
	"FLAsHZTRcf3Dy1APaz6j74ebdMC6Xx4g6i9YxjyrDybR",
}

var blockRazorTipAccounts = []string{
	"FjmZZrFvhnqqb9ThCuMVnENaM3JGVuGWNyCAxRJcFpg9",
	"6No2i3aawzHsjtThw81iq1EXPJN6rh8eSJCLaYZfKDTG",
	"A9cWowVAiHe9pJfKAj3TJiN9VpbzMUq6E4kEvf5mUT22",
	"Gywj98ophM7GmkDdaWs4isqZnDdFCW7B46TXmKfvyqSm",
	"68Pwb4jS7eZATjDfhmTXgRJjCiZmw1L7Huy4HNpnxJ3o",
	"4ABhJh5rZPjv63RBJBuyWzBK3g9gWMUQdTZP2kiW31V9",
	"B2M4NG5eyZp5SBQrSdtemzk5TqVuaWGQnowGaCBt8GyM",
	"5jA59cXMKQqZAVdtopv8q3yyw9SYfiE3vUCbt7p8MfVf",
	"5YktoWygr1Bp9wiS1xtMtUki1PeYuuzuCF98tqwYxf61",
	"295Avbam4qGShBYK7E9H5Ldew4B3WyJGmgmXfiWdeeyV",
	"EDi4rSy2LZgKJX74mbLTFk4mxoTgT6F7HxxzG2HBAFyK",
	"BnGKHAC386n4Qmv9xtpBVbRaUTKixjBe3oagkPFKtoy6",
	"Dd7K2Fp7AtoN8xCghKDRmyqr5U169t48Tw5fEd3wT9mq",
	"AP6qExwrbRgBAVaehg4b5xHENX815sMabtBzUzVB4v8S",
}

var astralaneTipAccounts = []string{
	"astrazznxsGUhWShqgNtAdfrzP2G83DzcWVJDxwV9bF",
	"astra4uejePWneqNaJKuFFA8oonqCE1sqF6b45kDMZm",
	"astra9xWY93QyfG6yM8zwsKsRodscjQ2uU2HKNL5prk",
	"astraRVUuTHjpwEVvNBeQEgwYx9w9CFyfxjYoobCZhL",
	"astraEJ2fEj8Xmy6KLG7B3VfbKfsHXhHrNdCQx7iGJK",
	"astraubkDw81n4LuutzSQ8uzHCv4BhPVhfvTcYv8SKC",
	"astraZW5GLFefxNPAatceHhYjfA1ciq9gvfEg2S47xk",
	"astrawVNP4xDBKT7rAdxrLYiTSTdqtUr63fSMduivXK",
	"AstrA1ejL4UeXC2SBP4cpeEmtcFPZVLxx3XGKXyCW6to",
	"AsTra79FET4aCKWspPqeSFvjJNyp96SvAnrmyAxqg5b7",
	"AstrABAu8CBTyuPXpV4eSCJ5fePEPnxN8NqBaPKQ9fHR",
	"AsTRADtvb6tTmrsqULQ9Wji9PigDMjhfEMza6zkynEvV",
	"AsTRAEoyMofR3vUPpf9k68Gsfb6ymTZttEtsAbv8Bk4d",
	"AStrAJv2RN2hKCHxwUMtqmSxgdcNZbihCwc1mCSnG83W",
	"Astran35aiQUF57XZsmkWMtNCtXGLzs8upfiqXxth2bz",
	"AStRAnpi6kFrKypragExgeRoJ1QnKH7pbSjLAKQVWUum",
	"ASTRaoF93eYt73TYvwtsv6fMWHWbGmMUZfVZPo3CRU9C",
}

var stelliumTipAccounts = []string{
	"ste11JV3MLMM7x7EJUM2sXcJC1H7F4jBLnP9a9PG8PH",
	"ste11MWPjXCRfQryCshzi86SGhuXjF4Lv6xMXD2AoSt",
	"ste11p5x8tJ53H1NbNQsRBg1YNRd4GcVpxtDw8PBpmb",
	"ste11p7e2KLYou5bwtt35H7BM6uMdo4pvioGjJXKFcN",
	"ste11TMV68LMi1BguM4RQujtbNCZvf1sjsASpqgAvSX",
}

var lightspeedTipAccounts = []string{
	"53PhM3UTdMQWu5t81wcd35AHGc5xpmHoRjem7GQPvXjA",
	"9tYF5yPDC1NP8s6diiB3kAX6ZZnva9DM3iDwJkBRarBB",
}

var soyasTipAccounts = []string{
	"soyas4s6L8KWZ8rsSk1mF3d1mQScoTGGAgjk98bF8nP",
	"soyascXFW5wEEYiwfEmHy2pNwomqzvggJosGVD6TJdY",
	"soyasDBdKjADwPz3xk82U3TNPRDKEWJj7wWLajNHZ1L",
	"soyasE2abjBAynmHbGWgEwk4ctBy7JMTUCNrMbjcnyH",
	"soyasi59njacMUPvo3TM5paHjeK8pYSdovXgFi32gRt",
	"soyasQYhJxv8uZgWDxhg72td6piAf7XTkoyWHtSATEz",
	"soyastP66xyYC8XADXZjdMM5BAVGD2YRvz8dwtLsqb8",
	"soyasvdgUJWYcUCzDxpmjUnNjH7KamXLXTzLwFvdVPE",
	"soyasvxAunisNxaoRxkKGjNir7KmbwYnr37JmefkX9G",
	"soyas5doVFUwH8s5zK8gEvCL5KR5ogDmf52LsrJEZ9h",
}

var speedlandingTipAccounts = []string{
	"SpEEdz8S1KorkMZqjMUxfxrmWwofmp6ReNP2Nx6CUmq",
	"SpeeDy3GJM4wcrQmk1itRFWgidvxX4rwjTLMv78wwjE",
	"SPeEdva37vW8vRtqgYjprQs1g3965icfVN5Rt7SMAyh",
	"speEdrSEpox5GUfHWcBc7tQjRuSfUin2yvB7qoYvvJh",
	"SPeEDmkHkN3A2roSZf6aZyEMsmrGqTHKqwP51y2Y4rV",
	"SpeedLdTJXh2RKpXEaP8JCxkWoUVXhtdPQ1EnxBJMxc",
	"SpEediGKLbbXndSYTzwmz6Z3NDgHQLDcTDEvGFkSMH9",
	"speede8xCcUq2Tiv1efXeTuE3k9TDNq8TnGKaKSc6J4",
}

var heliusTipAccounts = []string{
	"4ACfpUFoaSD9bfPdeu6DBt89gB6ENTeHBXCAi87NhDEE",
	"D2L6yPZ2FmmmTKPgzaMKdhu6EWZcTpLy1Vhx8uvZe7NZ",
	"9bnz4RShgq1hAnLnZbP8kbgBg1kEmcJBYQq3gQbmnSta",
	"5VY91ws6B2hMmBFRsXkoAAdsPHBJwRfBht4DXox3xkwn",
	"2nyhqdwKcJZR2vcqCyrYsaPVdAnFoJjiksCXJ7hfEYgD",
	"2q5pghRs6arqVjRvT5gfgWfWcHWmw1ZuCzphgd5KfWGJ",
	"wyvPkWjVZz1M8fHQnMMCDTQDbkManefNNhweYk5WkcF",
	"3KCKozbAaF75qEU33jtzozcJ29yJuaLJTy2jFdzUY8bT",
	"4vieeGHPYPG2MmyPRcYjdiDmmhN3ww7hsFNap8pVN3Ey",
	"4TQLFNWK8AovT1gFvda5jfw2oJeRMKEmw7aH6MGBJ3or",
}

// ===== Endpoint Maps =====

var jitoEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "https://ny.mainnet.block-engine.jito.wtf",
	SwqosRegionFrankfurt:  "https://frankfurt.mainnet.block-engine.jito.wtf",
	SwqosRegionAmsterdam:  "https://amsterdam.mainnet.block-engine.jito.wtf",
	SwqosRegionSLC:        "https://slc.mainnet.block-engine.jito.wtf",
	SwqosRegionTokyo:      "https://tokyo.mainnet.block-engine.jito.wtf",
	SwqosRegionLondon:     "https://london.mainnet.block-engine.jito.wtf",
	SwqosRegionLosAngeles: "https://ny.mainnet.block-engine.jito.wtf",
	SwqosRegionDefault:    "https://mainnet.block-engine.jito.wtf",
}

var nextBlockEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ny.nextblock.io",
	SwqosRegionFrankfurt:  "http://frankfurt.nextblock.io",
	SwqosRegionAmsterdam:  "http://amsterdam.nextblock.io",
	SwqosRegionSLC:        "http://slc.nextblock.io",
	SwqosRegionTokyo:      "http://tokyo.nextblock.io",
	SwqosRegionLondon:     "http://london.nextblock.io",
	SwqosRegionLosAngeles: "http://singapore.nextblock.io",
	SwqosRegionDefault:    "http://frankfurt.nextblock.io",
}

var zeroSlotEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ny.0slot.trade",
	SwqosRegionFrankfurt:  "http://de2.0slot.trade",
	SwqosRegionAmsterdam:  "http://ams.0slot.trade",
	SwqosRegionSLC:        "http://ny.0slot.trade",
	SwqosRegionTokyo:      "http://jp.0slot.trade",
	SwqosRegionLondon:     "http://ams.0slot.trade",
	SwqosRegionLosAngeles: "http://la.0slot.trade",
	SwqosRegionDefault:    "http://de2.0slot.trade",
}

var temporalEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ewr1.nozomi.temporal.xyz",
	SwqosRegionFrankfurt:  "http://fra2.nozomi.temporal.xyz",
	SwqosRegionAmsterdam:  "http://ams1.nozomi.temporal.xyz",
	SwqosRegionSLC:        "http://ewr1.nozomi.temporal.xyz",
	SwqosRegionTokyo:      "http://tyo1.nozomi.temporal.xyz",
	SwqosRegionLondon:     "http://sgp1.nozomi.temporal.xyz",
	SwqosRegionLosAngeles: "http://pit1.nozomi.temporal.xyz",
	SwqosRegionDefault:    "http://fra2.nozomi.temporal.xyz",
}

var bloxrouteEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "https://ny.solana.dex.blxrbdn.com",
	SwqosRegionFrankfurt:  "https://germany.solana.dex.blxrbdn.com",
	SwqosRegionAmsterdam:  "https://amsterdam.solana.dex.blxrbdn.com",
	SwqosRegionSLC:        "https://ny.solana.dex.blxrbdn.com",
	SwqosRegionTokyo:      "https://tokyo.solana.dex.blxrbdn.com",
	SwqosRegionLondon:     "https://uk.solana.dex.blxrbdn.com",
	SwqosRegionLosAngeles: "https://la.solana.dex.blxrbdn.com",
	SwqosRegionDefault:    "https://global.solana.dex.blxrbdn.com",
}

var node1Endpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ny.node1.me",
	SwqosRegionFrankfurt:  "http://fra.node1.me",
	SwqosRegionAmsterdam:  "http://ams.node1.me",
	SwqosRegionSLC:        "http://ny.node1.me",
	SwqosRegionTokyo:      "http://tk.node1.me",
	SwqosRegionLondon:     "http://lon.node1.me",
	SwqosRegionLosAngeles: "http://ny.node1.me",
	SwqosRegionDefault:    "http://fra.node1.me",
}

var flashBlockEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ny.flashblock.trade",
	SwqosRegionFrankfurt:  "http://fra.flashblock.trade",
	SwqosRegionAmsterdam:  "http://ams.flashblock.trade",
	SwqosRegionSLC:        "http://slc.flashblock.trade",
	SwqosRegionTokyo:      "http://singapore.flashblock.trade",
	SwqosRegionLondon:     "http://london.flashblock.trade",
	SwqosRegionLosAngeles: "http://ny.flashblock.trade",
	SwqosRegionDefault:    "http://ny.flashblock.trade",
}

var blockRazorEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://newyork.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionFrankfurt:  "http://frankfurt.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionAmsterdam:  "http://amsterdam.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionSLC:        "http://newyork.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionTokyo:      "http://tokyo.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionLondon:     "http://london.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionLosAngeles: "http://newyork.solana.blockrazor.xyz:443/v2/sendTransaction",
	SwqosRegionDefault:    "http://frankfurt.solana.blockrazor.xyz:443/v2/sendTransaction",
}

var astralaneEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ny.gateway.astralane.io/irisb",
	SwqosRegionFrankfurt:  "http://fr.gateway.astralane.io/irisb",
	SwqosRegionAmsterdam:  "http://ams.gateway.astralane.io/irisb",
	SwqosRegionSLC:        "http://ny.gateway.astralane.io/irisb",
	SwqosRegionTokyo:      "http://jp.gateway.astralane.io/irisb",
	SwqosRegionLondon:     "http://ny.gateway.astralane.io/irisb",
	SwqosRegionLosAngeles: "http://lax.gateway.astralane.io/irisb",
	SwqosRegionDefault:    "http://lim.gateway.astralane.io/irisb",
}

var stelliumEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ewr1.flashrpc.com",
	SwqosRegionFrankfurt:  "http://fra1.flashrpc.com",
	SwqosRegionAmsterdam:  "http://ams1.flashrpc.com",
	SwqosRegionSLC:        "http://ewr1.flashrpc.com",
	SwqosRegionTokyo:      "http://tyo1.flashrpc.com",
	SwqosRegionLondon:     "http://lhr1.flashrpc.com",
	SwqosRegionLosAngeles: "http://ewr1.flashrpc.com",
	SwqosRegionDefault:    "http://fra1.flashrpc.com",
}

var heliusEndpoints = map[SwqosRegion]string{
	SwqosRegionNewYork:    "http://ewr-sender.helius-rpc.com/fast",
	SwqosRegionFrankfurt:  "http://fra-sender.helius-rpc.com/fast",
	SwqosRegionAmsterdam:  "http://ams-sender.helius-rpc.com/fast",
	SwqosRegionSLC:        "http://slc-sender.helius-rpc.com/fast",
	SwqosRegionTokyo:      "http://tyo-sender.helius-rpc.com/fast",
	SwqosRegionLondon:     "http://lon-sender.helius-rpc.com/fast",
	SwqosRegionLosAngeles: "http://sg-sender.helius-rpc.com/fast",
	SwqosRegionDefault:    "https://sender.helius-rpc.com/fast",
}

// ===== Helper =====

func randomTipAccount(accounts []string) string {
	return accounts[rand.Intn(len(accounts))]
}

// ===== Interfaces =====

// SwqosClient defines the interface for SWQOS clients
type SwqosClient interface {
	SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error)
	SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error)
	GetTipAccount() string
	GetSwqosType() SwqosType
	MinTipSol() float64
}

// TradeError represents a trade error
type TradeError struct {
	Code       uint32
	Message    string
	Instruction *uint8
}

func (e *TradeError) Error() string {
	return e.Message
}

// ===== HTTP Client =====

var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     300 * time.Second,
			},
		}
	})
	return httpClient
}

// parseSignatureFromResult parses JSON-RPC result for a signature
func parseSignatureFromResult(body []byte) (solana.Signature, error) {
	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}
	if result.Error.Message != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error.Message}
	}
	return solana.SignatureFromBase58(result.Result)
}

// ===== Jito Client =====

// JitoClient represents a Jito SWQOS client
type JitoClient struct {
	endpoint  string
	authToken string
}

// NewJitoClient creates a new Jito client
func NewJitoClient(endpoint, authToken string) *JitoClient {
	return &JitoClient{
		endpoint:  endpoint,
		authToken: authToken,
	}
}

// SendTransaction sends a single transaction via Jito (/api/v1/transactions)
func (c *JitoClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)

	url := c.endpoint + "/api/v1/transactions"
	if c.authToken != "" {
		url += "?uuid=" + c.authToken
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("x-jito-auth", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

// SendTransactions sends multiple transactions as a Jito bundle (/api/v1/bundles)
func (c *JitoClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	encodedTxs := make([]string, len(transactions))
	for i, tx := range transactions {
		encodedTxs[i] = base64.StdEncoding.EncodeToString(tx)
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendBundle",
		"params": []interface{}{
			encodedTxs,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)

	url := c.endpoint + "/api/v1/bundles"
	if c.authToken != "" {
		url += "?uuid=" + c.authToken
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("x-jito-auth", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	sig, err := parseSignatureFromResult(body)
	if err != nil {
		return nil, err
	}
	return []solana.Signature{sig}, nil
}

func (c *JitoClient) GetTipAccount() string    { return randomTipAccount(jitoTipAccounts) }
func (c *JitoClient) GetSwqosType() SwqosType  { return SwqosTypeJito }
func (c *JitoClient) MinTipSol() float64       { return MinTipJito }

// ===== NextBlock Client =====

// NextBlockClient represents a NextBlock SWQOS client
type NextBlockClient struct {
	endpoint  string
	authToken string
}

// NewNextBlockClientHTTP creates a new NextBlock client
func NewNextBlockClientHTTP(endpoint, authToken string) *NextBlockClient {
	return &NextBlockClient{endpoint: endpoint, authToken: authToken}
}

func (c *NextBlockClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction":          map[string]interface{}{"content": encoded},
		"frontRunningProtection": false,
	}

	jsonData, _ := json.Marshal(payload)
	url := c.endpoint + "/api/v2/submit"

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}
	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}
	return solana.SignatureFromBase58(result.Signature)
}

func (c *NextBlockClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *NextBlockClient) GetTipAccount() string    { return randomTipAccount(nextBlockTipAccounts) }
func (c *NextBlockClient) GetSwqosType() SwqosType  { return SwqosTypeNextBlock }
func (c *NextBlockClient) MinTipSol() float64       { return MinTipNextBlock }

// ===== ZeroSlot Client =====

// ZeroSlotClient represents a ZeroSlot SWQOS client
type ZeroSlotClient struct {
	endpoint  string
	authToken string
}

// NewZeroSlotClient creates a new ZeroSlot client
func NewZeroSlotClient(endpoint, authToken string) *ZeroSlotClient {
	return &ZeroSlotClient{endpoint: endpoint, authToken: authToken}
}

func (c *ZeroSlotClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)
	url := c.endpoint + "/?api-key=" + c.authToken

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *ZeroSlotClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *ZeroSlotClient) GetTipAccount() string    { return randomTipAccount(zeroSlotTipAccounts) }
func (c *ZeroSlotClient) GetSwqosType() SwqosType  { return SwqosTypeZeroSlot }
func (c *ZeroSlotClient) MinTipSol() float64       { return MinTipZeroSlot }

// ===== Temporal Client =====

// TemporalClient represents a Temporal SWQOS client
type TemporalClient struct {
	endpoint  string
	authToken string
}

// NewTemporalClient creates a new Temporal client
func NewTemporalClient(endpoint, authToken string) *TemporalClient {
	return &TemporalClient{endpoint: endpoint, authToken: authToken}
}

func (c *TemporalClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)
	url := c.endpoint + "/?c=" + c.authToken

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *TemporalClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *TemporalClient) GetTipAccount() string    { return randomTipAccount(temporalTipAccounts) }
func (c *TemporalClient) GetSwqosType() SwqosType  { return SwqosTypeTemporal }
func (c *TemporalClient) MinTipSol() float64       { return MinTipTemporal }

// ===== Bloxroute Client =====

// BloxrouteClient represents a Bloxroute SWQOS client
type BloxrouteClient struct {
	endpoint  string
	authToken string
}

// NewBloxrouteClient creates a new Bloxroute client
func NewBloxrouteClient(endpoint, authToken string) *BloxrouteClient {
	return &BloxrouteClient{endpoint: endpoint, authToken: authToken}
}

func (c *BloxrouteClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transaction":      map[string]interface{}{"content": encoded},
		"frontRunningProtection": false,
		"useStakedRPCs":    true,
	}

	jsonData, _ := json.Marshal(payload)
	url := c.endpoint + "/api/v2/submit"

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authToken)

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}
	if result.Reason != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Reason}
	}
	return solana.SignatureFromBase58(result.Signature)
}

func (c *BloxrouteClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *BloxrouteClient) GetTipAccount() string    { return randomTipAccount(bloxrouteTipAccounts) }
func (c *BloxrouteClient) GetSwqosType() SwqosType  { return SwqosTypeBloxroute }
func (c *BloxrouteClient) MinTipSol() float64       { return MinTipBloxroute }

// ===== Node1 Client =====

// Node1Client represents a Node1 SWQOS client
type Node1Client struct {
	endpoint  string
	authToken string
}

// NewNode1Client creates a new Node1 client
func NewNode1Client(endpoint, authToken string) *Node1Client {
	return &Node1Client{endpoint: endpoint, authToken: authToken}
}

func (c *Node1Client) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding":      "base64",
				"skipPreflight": true,
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("api-key", c.authToken)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *Node1Client) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *Node1Client) GetTipAccount() string    { return randomTipAccount(node1TipAccounts) }
func (c *Node1Client) GetSwqosType() SwqosType  { return SwqosTypeNode1 }
func (c *Node1Client) MinTipSol() float64       { return MinTipNode1 }

// ===== FlashBlock Client =====

// FlashBlockClient represents a FlashBlock SWQOS client
type FlashBlockClient struct {
	endpoint  string
	authToken string
}

// NewFlashBlockClient creates a new FlashBlock client
func NewFlashBlockClient(endpoint, authToken string) *FlashBlockClient {
	return &FlashBlockClient{endpoint: endpoint, authToken: authToken}
}

func (c *FlashBlockClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"transactions": []string{encoded},
	}

	jsonData, _ := json.Marshal(payload)
	url := c.endpoint + "/api/v2/submit-batch"

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authToken)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=30, max=1000")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return solana.Signature{}, err
	}
	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}
	return solana.SignatureFromBase58(result.Signature)
}

func (c *FlashBlockClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *FlashBlockClient) GetTipAccount() string    { return randomTipAccount(flashBlockTipAccounts) }
func (c *FlashBlockClient) GetSwqosType() SwqosType  { return SwqosTypeFlashBlock }
func (c *FlashBlockClient) MinTipSol() float64       { return MinTipFlashBlock }

// ===== BlockRazor Client =====

// BlockRazorClient represents a BlockRazor SWQOS client
type BlockRazorClient struct {
	endpoint      string
	authToken     string
	mevProtection bool
}

// NewBlockRazorClient creates a new BlockRazor client
func NewBlockRazorClient(endpoint, authToken string, mevProtection bool) *BlockRazorClient {
	return &BlockRazorClient{
		endpoint:      endpoint,
		authToken:     authToken,
		mevProtection: mevProtection,
	}
}

func (c *BlockRazorClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	mode := "fast"
	if c.mevProtection {
		mode = "sandwichMitigation"
	}

	url := fmt.Sprintf("%s?auth=%s&mode=%s", c.endpoint, c.authToken, mode)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(encoded))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Signature string `json:"signature"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// Try parsing as plain signature string
		sig := strings.TrimSpace(string(body))
		return solana.SignatureFromBase58(sig)
	}
	if result.Error != "" {
		return solana.Signature{}, &TradeError{Code: 500, Message: result.Error}
	}
	return solana.SignatureFromBase58(result.Signature)
}

func (c *BlockRazorClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *BlockRazorClient) GetTipAccount() string    { return randomTipAccount(blockRazorTipAccounts) }
func (c *BlockRazorClient) GetSwqosType() SwqosType  { return SwqosTypeBlockRazor }
func (c *BlockRazorClient) MinTipSol() float64       { return MinTipBlockRazor }

// ===== Astralane Client =====

// AstralaneClient represents an Astralane SWQOS client
type AstralaneClient struct {
	endpoint  string
	authToken string
}

// NewAstralaneClient creates a new Astralane client
func NewAstralaneClient(endpoint, authToken string) *AstralaneClient {
	return &AstralaneClient{endpoint: endpoint, authToken: authToken}
}

func (c *AstralaneClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	// Use JSON-RPC format as fallback (native Astralane uses bincode over HTTP/QUIC)
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s?api-key=%s&method=sendTransaction", c.endpoint, c.authToken)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *AstralaneClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *AstralaneClient) GetTipAccount() string    { return randomTipAccount(astralaneTipAccounts) }
func (c *AstralaneClient) GetSwqosType() SwqosType  { return SwqosTypeAstralane }
func (c *AstralaneClient) MinTipSol() float64       { return MinTipAstralane }

// ===== Stellium Client =====

// StelliumClient represents a Stellium SWQOS client
type StelliumClient struct {
	endpoint  string
	authToken string
}

// NewStelliumClient creates a new Stellium client
func NewStelliumClient(endpoint, authToken string) *StelliumClient {
	return &StelliumClient{endpoint: endpoint, authToken: authToken}
}

func (c *StelliumClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)
	// Stellium: API key is part of the URL path
	url := fmt.Sprintf("%s/%s", c.endpoint, c.authToken)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=30, max=1000")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *StelliumClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *StelliumClient) GetTipAccount() string    { return randomTipAccount(stelliumTipAccounts) }
func (c *StelliumClient) GetSwqosType() SwqosType  { return SwqosTypeStellium }
func (c *StelliumClient) MinTipSol() float64       { return MinTipStellium }

// ===== Lightspeed Client =====

// LightspeedClient represents a Lightspeed (Solana Vibe Station) SWQOS client
type LightspeedClient struct {
	endpoint string // full URL including /lightspeed?api_key=...
}

// NewLightspeedClient creates a new Lightspeed client
// endpoint should be the full URL e.g. https://starter.rpc.solanavibestation.com/lightspeed?api_key=TOKEN
func NewLightspeedClient(endpoint string) *LightspeedClient {
	return &LightspeedClient{endpoint: endpoint}
}

func (c *LightspeedClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding":            "base64",
				"skipPreflight":       true,
				"preflightCommitment": "processed",
				"maxRetries":          0,
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *LightspeedClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *LightspeedClient) GetTipAccount() string    { return randomTipAccount(lightspeedTipAccounts) }
func (c *LightspeedClient) GetSwqosType() SwqosType  { return SwqosTypeLightspeed }
func (c *LightspeedClient) MinTipSol() float64       { return MinTipLightspeed }

// ===== Soyas Client =====

// SoyasClient represents a Soyas SWQOS client (QUIC-based, HTTP fallback)
type SoyasClient struct {
	endpoint  string // QUIC endpoint host:port
	authToken string // base58 keypair used for mTLS
}

// NewSoyasClient creates a new Soyas client
func NewSoyasClient(endpoint, authToken string) *SoyasClient {
	return &SoyasClient{endpoint: endpoint, authToken: authToken}
}

// SendTransaction submits via standard RPC as fallback (native is QUIC/mTLS)
func (c *SoyasClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	// Soyas uses QUIC with mTLS - HTTP submission not natively supported.
	// Return not-implemented error; callers should use QUIC transport directly.
	return solana.Signature{}, &TradeError{Code: 501, Message: "Soyas requires QUIC transport"}
}

func (c *SoyasClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	return nil, &TradeError{Code: 501, Message: "Soyas requires QUIC transport"}
}

func (c *SoyasClient) GetTipAccount() string    { return randomTipAccount(soyasTipAccounts) }
func (c *SoyasClient) GetSwqosType() SwqosType  { return SwqosTypeSoyas }
func (c *SoyasClient) MinTipSol() float64       { return MinTipSoyas }

// ===== Speedlanding Client =====

// SpeedlandingClient represents a Speedlanding SWQOS client (QUIC-based, HTTP fallback)
type SpeedlandingClient struct {
	endpoint  string // QUIC endpoint host:port
	authToken string // base58 keypair used for mTLS
}

// NewSpeedlandingClient creates a new Speedlanding client
func NewSpeedlandingClient(endpoint, authToken string) *SpeedlandingClient {
	return &SpeedlandingClient{endpoint: endpoint, authToken: authToken}
}

// SendTransaction submits via QUIC - returns not-implemented for HTTP callers
func (c *SpeedlandingClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	return solana.Signature{}, &TradeError{Code: 501, Message: "Speedlanding requires QUIC transport"}
}

func (c *SpeedlandingClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	return nil, &TradeError{Code: 501, Message: "Speedlanding requires QUIC transport"}
}

func (c *SpeedlandingClient) GetTipAccount() string    { return randomTipAccount(speedlandingTipAccounts) }
func (c *SpeedlandingClient) GetSwqosType() SwqosType  { return SwqosTypeSpeedlanding }
func (c *SpeedlandingClient) MinTipSol() float64       { return MinTipSpeedlanding }

// ===== Helius Client =====

// HeliusClient represents a Helius SWQOS client
type HeliusClient struct {
	url       string // pre-built URL with api-key and swqos_only params
	swqosOnly bool
}

// NewHeliusClient creates a new Helius client
func NewHeliusClient(endpoint, apiKey string, swqosOnly bool) *HeliusClient {
	url := endpoint
	if apiKey != "" {
		url += "?api-key=" + apiKey
		if swqosOnly {
			url += "&swqos_only=true"
		}
	} else if swqosOnly {
		url += "?swqos_only=true"
	}
	return &HeliusClient{url: url, swqosOnly: swqosOnly}
}

func (c *HeliusClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1", // string "1", not integer
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{
				"encoding":      "base64",
				"skipPreflight": true,
				"maxRetries":    0,
			},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *HeliusClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *HeliusClient) GetTipAccount() string    { return randomTipAccount(heliusTipAccounts) }
func (c *HeliusClient) GetSwqosType() SwqosType  { return SwqosTypeHelius }
func (c *HeliusClient) MinTipSol() float64 {
	if c.swqosOnly {
		return MinTipHelius
	}
	return 0.0002
}

// ===== Default RPC Client =====

// DefaultClient represents a default RPC client
type DefaultClient struct {
	rpcURL string
}

// NewDefaultClient creates a new default client
func NewDefaultClient(rpcURL string) *DefaultClient {
	return &DefaultClient{rpcURL: rpcURL}
}

func (c *DefaultClient) SendTransaction(ctx context.Context, tradeType TradeType, transaction []byte, waitConfirmation bool) (solana.Signature, error) {
	encoded := base64.StdEncoding.EncodeToString(transaction)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			encoded,
			map[string]interface{}{"encoding": "base64"},
		},
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return solana.Signature{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return solana.Signature{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseSignatureFromResult(body)
}

func (c *DefaultClient) SendTransactions(ctx context.Context, tradeType TradeType, transactions [][]byte, waitConfirmation bool) ([]solana.Signature, error) {
	sigs := make([]solana.Signature, 0, len(transactions))
	for _, tx := range transactions {
		sig, err := c.SendTransaction(ctx, tradeType, tx, waitConfirmation)
		if err != nil {
			return sigs, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

func (c *DefaultClient) GetTipAccount() string    { return "" }
func (c *DefaultClient) GetSwqosType() SwqosType  { return SwqosTypeDefault }
func (c *DefaultClient) MinTipSol() float64       { return MinTipDefault }

// ===== GetAllSwqosTypes =====

// GetAllSwqosTypes returns all SWQOS types
func GetAllSwqosTypes() []SwqosType {
	return []SwqosType{
		SwqosTypeJito, SwqosTypeNextBlock, SwqosTypeZeroSlot, SwqosTypeTemporal,
		SwqosTypeBloxroute, SwqosTypeNode1, SwqosTypeFlashBlock, SwqosTypeBlockRazor,
		SwqosTypeAstralane, SwqosTypeStellium, SwqosTypeLightspeed, SwqosTypeSoyas,
		SwqosTypeSpeedlanding, SwqosTypeHelius, SwqosTypeDefault,
	}
}

// ===== Client Factory =====

// ClientFactory creates SWQOS clients based on config
type ClientFactory struct{}

// CreateClient creates a SWQOS client from config
func (f *ClientFactory) CreateClient(config soltradesdk.SwqosConfig, rpcURL string) (SwqosClient, error) {
	switch config.Type {
	case SwqosTypeJito:
		endpoint, ok := jitoEndpoints[config.Region]
		if !ok {
			endpoint = jitoEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewJitoClient(endpoint, config.APIKey), nil

	case SwqosTypeNextBlock:
		endpoint, ok := nextBlockEndpoints[config.Region]
		if !ok {
			endpoint = nextBlockEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewNextBlockClientHTTP(endpoint, config.APIKey), nil

	case SwqosTypeZeroSlot:
		endpoint, ok := zeroSlotEndpoints[config.Region]
		if !ok {
			endpoint = zeroSlotEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewZeroSlotClient(endpoint, config.APIKey), nil

	case SwqosTypeTemporal:
		endpoint, ok := temporalEndpoints[config.Region]
		if !ok {
			endpoint = temporalEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewTemporalClient(endpoint, config.APIKey), nil

	case SwqosTypeBloxroute:
		endpoint, ok := bloxrouteEndpoints[config.Region]
		if !ok {
			endpoint = bloxrouteEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewBloxrouteClient(endpoint, config.APIKey), nil

	case SwqosTypeNode1:
		endpoint, ok := node1Endpoints[config.Region]
		if !ok {
			endpoint = node1Endpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewNode1Client(endpoint, config.APIKey), nil

	case SwqosTypeFlashBlock:
		endpoint, ok := flashBlockEndpoints[config.Region]
		if !ok {
			endpoint = flashBlockEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewFlashBlockClient(endpoint, config.APIKey), nil

	case SwqosTypeBlockRazor:
		endpoint, ok := blockRazorEndpoints[config.Region]
		if !ok {
			endpoint = blockRazorEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewBlockRazorClient(endpoint, config.APIKey, config.MEVProtection), nil

	case SwqosTypeAstralane:
		endpoint, ok := astralaneEndpoints[config.Region]
		if !ok {
			endpoint = astralaneEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewAstralaneClient(endpoint, config.APIKey), nil

	case SwqosTypeStellium:
		endpoint, ok := stelliumEndpoints[config.Region]
		if !ok {
			endpoint = stelliumEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewStelliumClient(endpoint, config.APIKey), nil

	case SwqosTypeLightspeed:
		// Lightspeed endpoint includes the full path with api_key
		endpoint := config.CustomURL
		if endpoint == "" {
			endpoint = fmt.Sprintf("https://starter.rpc.solanavibestation.com/lightspeed?api_key=%s", config.APIKey)
		}
		return NewLightspeedClient(endpoint), nil

	case SwqosTypeSoyas:
		// Soyas uses QUIC; endpoint is host:port
		endpoint := "fra.landing.soyas.xyz:9000"
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewSoyasClient(endpoint, config.APIKey), nil

	case SwqosTypeSpeedlanding:
		// Speedlanding uses QUIC; endpoint is host:port
		endpoint := "fra.speedlanding.trade:17778"
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewSpeedlandingClient(endpoint, config.APIKey), nil

	case SwqosTypeHelius:
		endpoint, ok := heliusEndpoints[config.Region]
		if !ok {
			endpoint = heliusEndpoints[SwqosRegionDefault]
		}
		if config.CustomURL != "" {
			endpoint = config.CustomURL
		}
		return NewHeliusClient(endpoint, config.APIKey, false), nil

	case SwqosTypeDefault:
		return NewDefaultClient(rpcURL), nil

	default:
		return nil, fmt.Errorf("unsupported SWQOS type: %v", config.Type)
	}
}
