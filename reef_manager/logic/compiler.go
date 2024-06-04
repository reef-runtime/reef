package logic

import (
	"context"
	"errors"
	"fmt"
	"net"

	"capnproto.org/go/capnp/v3/rpc"
	compiler "github.com/reef-runtime/reef/reef_protocol_compiler"
)

const compilerServiceDialType = "tcp"

type CompilerConfig struct {
	IP   string `env:"REEF_COMPILER_IP"   env-required:"true"`
	Port uint16 `env:"REEF_COMPILER_PORT" env-required:"true"`
}

type CompilerManager struct {
	config CompilerConfig
	conn   *rpc.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

type CompilationError *string

var CompilerInstance CompilerManager

//
// Instance management.
//

func InitCompiler(config CompilerConfig) {
	CompilerInstance = newCompiler(config)
}

func DestroyCompiler() error {
	return CompilerInstance.destroy()
}

//
// Generic, not specific to the singleton instance.
//

func (c *CompilerManager) destroy() error {
	return c.conn.Close()
}

func newCompiler(
	config CompilerConfig,
) CompilerManager {
	log.Debugf("Connecting to remote compiler at %s:%d...", config.IP, config.Port)
	ctx, cancel := context.WithCancel(context.Background())

	netConn, err := net.Dial(compilerServiceDialType, fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		panic(err.Error())
	}

	rpcConn := rpc.NewConn(rpc.NewStreamTransport(netConn), nil)

	log.Info("Connected to compiler service")

	return CompilerManager{
		config: config,
		conn:   rpcConn,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *CompilerManager) Compile(
	language compiler.Language,
	programSrc string,
) (wasm []byte, comp CompilationError, system error) {
	rpcCompiler := compiler.Compiler(c.conn.Bootstrap(c.ctx))
	f, release := rpcCompiler.Compile(c.ctx, func(c compiler.Compiler_compile_Params) error {
		c.SetLanguage(language)
		return c.SetProgramSrc(programSrc)
	})
	defer release()
	defer rpcCompiler.Release()

	res, err := f.Struct()
	if err != nil {
		return nil, nil, err
	}

	r, err := res.Response()
	if err != nil {
		return nil, nil, err
	}

	switch r.Which() {
	case compiler.CompilerResponse_Which_fileContent:
		file, err := r.FileContent()
		if err != nil {
			return nil, nil, err
		}

		return file, nil, nil
	case compiler.CompilerResponse_Which_systemError:
		fs, err := r.SystemError()
		if err != nil {
			return nil, nil, err
		}

		return nil, nil, errors.New(fs)
	case compiler.CompilerResponse_Which_compilerError:
		cpE, err := r.CompilerError()
		if err != nil {
			return nil, nil, err
		}

		return nil, &cpE, err
	default:
		panic("Unreachable: a new type of compiler reply occurred")
	}
}
