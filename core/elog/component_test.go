package elog

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/gotomicro/ego/core/econf"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestRotateLogger(t *testing.T) {
	err := os.Setenv("EGO_DEBUG", "false")
	assert.NoError(t, err)
	conf := `
[default]
debug = false
level = "info"
enableAsync = false
`
	err = econf.LoadFromReader(strings.NewReader(conf), toml.Unmarshal)
	assert.NoError(t, err)
	logger := Load("default").Build()
	defer logger.Flush()
	logger.Error("1")

	childLogger := logger.With(String("prefix", "PREFIX"))
	childLogger.Error("2")
	defer childLogger.Flush()

	logger.Error("3")
	logger.With(String("prefix2", "PREFIX2"))
	logger.Error("4")
}

var messages = fakeMessages(1000)

func newFileLogger(path string) *Component {
	conf := `
[file]
level = "info"
name = "%s"
`
	conf = fmt.Sprintf(conf, path)
	var err error
	if err = econf.LoadFromReader(strings.NewReader(conf), toml.Unmarshal); err != nil {
		log.Println("load conf fail", err)
		return nil
	}
	log.Println("start to send logs to file")
	return Load("file").Build()
}

func newStderrLogger() *Component {
	conf := `
[stderr]
level = "info"
writer = "stderr"
`
	var err error
	if err = econf.LoadFromReader(strings.NewReader(conf), toml.Unmarshal); err != nil {
		log.Println("load conf fail", err)
		return nil
	}
	log.Println("start to send logs to stderr")
	return Load("stderr").Build()
}

func newAliLogger() *Component {
	conf := `
[ali]
level = "info"
enableAsync = false
writer = "ali"
flushBufferSize = 2097152     # flushBufferSize set to 2MB
flushBufferInterval = "2s"
aliEndpoint = "%s"            # your ali sls endpoint
aliAccessKeyID = "%s"         # your ali sls AK ID
aliAccessKeySecret = "%s"     # your ali sls AK Secret
aliProject = "%s"             # your ali sls project
aliLogstore = "%s"            # your ali logstore
aliApiBulkSize = 512          # al api bulk size
aliApiTimeout = "3s"          # ali api timeout
aliApiRetryCount = 3          # ali api retry
aliApiRetryWaitTime = "1s"    # ali api retry wait time
aliApiRetryMaxWaitTime = "3s" # ali api retry wait max wait time
aliApiMaxIdleConnsPerHost = 20
aliApiMaxIdleConns = 25
`
	conf = fmt.Sprintf(conf,
		os.Getenv("ALI_ENDPOINT"),
		os.Getenv("ALI_AK_ID"),
		os.Getenv("ALI_AK_SECRET"),
		os.Getenv("ALI_PROJECT"),
		os.Getenv("ALI_LOGSTORE"),
	)
	var err error
	if err = econf.LoadFromReader(strings.NewReader(conf), toml.Unmarshal); err != nil {
		log.Println("load conf fail", err)
		return nil
	}
	log.Println("start to send logs to ali sls")

	return Load("ali").Build()
}

func fakeMessages(n int) []string {
	messages := make([]string, n)
	for i := range messages {
		messages[i] = fmt.Sprintf("Test logging, but use a somewhat realistic message length. (#%v)", i)
	}
	return messages
}

func getMessage(iter int) string {
	return messages[iter%1000]
}

func BenchmarkStderrWriter(b *testing.B) {
	b.Logf("Logging at a disabled level with some accumulated context.")
	logger1 := newStderrLogger()
	b.Run("stderr", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger1.Info(getMessage(0))
			}
		})
	})
}

func BenchmarkAliWriter(b *testing.B) {
	b.Logf("Logging at a disabled level with some accumulated context.")
	aliLogger := newAliLogger()
	b.Run("ali", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				aliLogger.Info(getMessage(0))
			}
		})
	})
}

func TestMultiLogger(t *testing.T) {
	os.RemoveAll("./logs")
	fileMultiLogger := newFileLogger("multi.log")
	fileMultiLoggers := []*Component{}
	for i := 0; i < 10; i++ {
		fileMultiLoggers = append(fileMultiLoggers, fileMultiLogger.With())
	}
	for i := 0; i < 10; i++ {
		for j := 0; j < 100000; j++ {
			fileMultiLoggers[i].Info(getMessage(0))
		}
	}

	for i := 0; i < 10; i++ {
		_ = fileMultiLoggers[i].Flush()
	}
	log.Println(`done--------------->`)
}

func BenchmarkMultiLogger(b *testing.B) {
	b.Logf("Logging at a single logger and multi child logger.")
	os.RemoveAll("./logs")
	fileMultiLogger := newFileLogger("child.log")
	fileMultiLoggers := []*Component{}
	for i := 0; i < 10; i++ {
		fileMultiLoggers = append(fileMultiLoggers, fileMultiLogger.With())
	}
	b.Run("child-file-logger", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				for i := 0; i < 10; i++ {
					fileMultiLoggers[i].Info(getMessage(0))
				}
			}
		})
	})

	fileSingleLoggers := []*Component{}
	for i := 0; i < 10; i++ {
		fileSingleLoggers = append(fileSingleLoggers, newFileLogger(fmt.Sprintf("independent-%d.log", i)))
	}
	b.Run("independent-file-logger", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				for i := 0; i < 10; i++ {
					fileSingleLoggers[i].Info(getMessage(0))
				}
			}
		})
	})

}

func Test_IsDebugMode(t *testing.T) {
	cmp := &Component{
		config: defaultConfig(),
	}
	cmp.config.Debug = true
	assert.True(t, cmp.IsDebugMode())
}

func TestSetLevel(t *testing.T) {
	logger := &Component{
		config: defaultConfig(),
	}
	logger.lv = &logger.config.al
	logger.SetLevel(zapcore.ErrorLevel)
	assert.Equal(t, "error", logger.lv.String())
}
