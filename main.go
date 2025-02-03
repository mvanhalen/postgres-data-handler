package main

import (
	"flag"

	"github.com/deso-protocol/core/lib"
	"github.com/deso-protocol/postgres-data-handler/handler"

	//"github.com/deso-protocol/postgres-data-handler/migrations/post_sync_migrations"
	"github.com/deso-protocol/state-consumer/consumer"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

func main() {
	// Initialize flags and get config values.
	setupFlags()
	stateChangeDir, consumerProgressDir, batchBytes, threadLimit, logQueries,
		explorerStatistics, datadogProfiler, isTestnet, isRegtest, isAcceleratedRegtest, syncMempool := getConfigValues()

	// Print all the config values in a single printf call broken up
	// with newlines and make it look pretty both printed out and in code
	glog.Infof(`
		PostgresDataHandler Config Values:
		---------------------------------
		STATE_CHANGE_DIR: %s
		CONSUMER_PROGRESS_DIR: %s
		BATCH_BYTES: %d
		THREAD_LIMIT: %d
		LOG_QUERIES: %t
		CALCULATE_EXPLORER_STATISTICS: %t
		DATA_DOG_PROFILER: %t
		TESTNET: %t
		`,
		stateChangeDir, consumerProgressDir, batchBytes, threadLimit,
		logQueries, explorerStatistics, datadogProfiler, isTestnet)

	// Initialize the DB.
	//db, err := setupDb(pgURI, threadLimit, logQueries, readOnlyUserPassword, explorerStatistics)
	// if err != nil {
	// 	glog.Fatalf("Error setting up DB: %v", err)
	// }
	//err :=
	// Setup profiler if enabled.
	if datadogProfiler {
		tracer.Start()
		err := profiler.Start(profiler.WithProfileTypes(profiler.CPUProfile, profiler.BlockProfile, profiler.MutexProfile, profiler.GoroutineProfile, profiler.HeapProfile))
		if err != nil {
			glog.Fatal(err)
		}
	}

	params := &lib.DeSoMainnetParams
	if isTestnet {
		params = &lib.DeSoTestnetParams
		if isRegtest {
			params.EnableRegtest(isAcceleratedRegtest)
		}
	}
	lib.GlobalDeSoParams = *params

	//cachedEntries, err := lru.New[string, []byte](int(handler.EntryCacheSize))
	// if err != nil {
	// 	glog.Fatalf("Error creating LRU cache: %v", err)
	// }

	// Initialize and run a state syncer consumer.
	// stateSyncerConsumer := &consumer.StateSyncerConsumer{}
	// err = stateSyncerConsumer.InitializeAndRun(
	// 	stateChangeDir,
	// 	consumerProgressDir,
	// 	batchBytes,
	// 	threadLimit,
	// 	syncMempool,
	// 	&handler.PostgresDataHandler{
	// 		DB:            db,
	// 		Params:        params,
	// 		CachedEntries: cachedEntries,
	// 	},
	// )

	// For instance, if you have a configuration value for minimum block height:
	minBlockHeight := uint64(100000) // Replace with your desired threshold.

	// Create the WebHandler with your desired transport settings and minimum block height.
	// For HTTP transport:
	webHandler := handler.NewWebHandler("https://your-api-endpoint.example.com/data", false, "", minBlockHeight)
	// For WebSocket, set useWebSocket to true and provide the WS URL:
	// webHandler := handler.NewWebHandler("", true, "wss://your-ws-endpoint.example.com/stream", minBlockHeight)

	// ... state change directory, consumer progress directory, batch bytes, thread limit, syncMempool, etc. ...
	// Pass webHandler to the consumer.
	stateSyncerConsumer := &consumer.StateSyncerConsumer{}
	err := stateSyncerConsumer.InitializeAndRun(
		stateChangeDir,
		consumerProgressDir,
		batchBytes,
		threadLimit,
		syncMempool,

		webHandler,
	)

	if err != nil {
		glog.Fatal(err)
	}
}

func setupFlags() {
	// Set glog flags
	flag.Set("log_dir", viper.GetString("log_dir"))
	flag.Set("v", viper.GetString("glog_v"))
	flag.Set("vmodule", viper.GetString("glog_vmodule"))
	flag.Set("alsologtostderr", "true")
	flag.Parse()
	glog.CopyStandardLogTo("INFO")
	viper.SetConfigFile(".env")
	viper.ReadInConfig()
	viper.AutomaticEnv()
}

func getConfigValues() (stateChangeDir string, consumerProgressDir string, batchBytes uint64, threadLimit int, logQueries bool, explorerStatistics bool, datadogProfiler bool, isTestnet bool, isRegtest bool, isAcceleratedRegtest bool, syncMempool bool) {

	// dbHost := viper.GetString("DB_HOST")
	// dbPort := viper.GetString("DB_PORT")
	// dbUsername := viper.GetString("DB_USERNAME")
	// dbPassword := viper.GetString("DB_PASSWORD")
	// dbName := "postgres"
	// if viper.GetString("DB_NAME") != "" {
	// 	dbName = viper.GetString("DB_NAME")
	// }

	// pgURI = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timeout=18000s", dbUsername, dbPassword, dbHost, dbPort, dbName)

	stateChangeDir = viper.GetString("STATE_CHANGE_DIR")
	if stateChangeDir == "" {
		stateChangeDir = "/tmp/state-changes"
	}
	// Set the state change dir flag that core uses, so DeSoEncoders properly encode and decode state change metadata.
	viper.Set("state-change-dir", stateChangeDir)

	consumerProgressDir = viper.GetString("CONSUMER_PROGRESS_DIR")
	if consumerProgressDir == "" {
		consumerProgressDir = "/tmp/consumer-progress"
	}

	batchBytes = viper.GetUint64("BATCH_BYTES")
	if batchBytes == 0 {
		batchBytes = 5000000
	}

	threadLimit = viper.GetInt("THREAD_LIMIT")
	if threadLimit == 0 {
		threadLimit = 25
	}

	syncMempool = viper.GetBool("SYNC_MEMPOOL")

	logQueries = viper.GetBool("LOG_QUERIES")
	//readonlyUserPassword = viper.GetString("READONLY_USER_PASSWORD")
	explorerStatistics = viper.GetBool("CALCULATE_EXPLORER_STATISTICS")
	datadogProfiler = viper.GetBool("DATADOG_PROFILER")
	isTestnet = viper.GetBool("IS_TESTNET")
	isRegtest = viper.GetBool("REGTEST")
	isAcceleratedRegtest = viper.GetBool("ACCELERATED_REGTEST")

	return stateChangeDir, consumerProgressDir, batchBytes, threadLimit, logQueries, explorerStatistics, datadogProfiler, isTestnet, isRegtest, isAcceleratedRegtest, syncMempool
}
