package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"

	"capnproto.org/go/capnp/v3/rpc"
	compiler "github.com/reef-runtime/reef/reef_protocol_compiler"
)

const compilerServiceDialType = "tcp"

type CompilerConfig struct {
	IP           string `env:"REEF_COMPILER_IP"            env-required:"true"`
	Port         uint16 `env:"REEF_COMPILER_PORT"          env-required:"true"`
	ArtifactPath string `env:"REEF_COMPILER_CACHE_PATH" env-required:"true"`
}

type CompilerManager struct {
	config CompilerConfig
	ctx    context.Context
	cancel context.CancelFunc
}

type CompilationError *string

// var CompilerInstance CompilerManager

//
// Instance management.
//

func NewCompiler(config CompilerConfig) (CompilerManager, error) {
	comp, err := newCompiler(config)
	if err != nil {
		return CompilerManager{}, err
	}

	return comp, nil
}

// func DestroyCompiler() error {
// 	return CompilerInstance.destroy()
// }

//
// Generic, not specific to the singleton instance.
//

type compilerConn struct {
	transport net.Conn
	rpc       *rpc.Conn
}

func (c *CompilerManager) connect() (compilerConn, error) {
	log.Debugf("Establishing connection to compiler at `%s:%d`", c.config.IP, c.config.Port)
	netConn, err := net.Dial(compilerServiceDialType, fmt.Sprintf("%s:%d", c.config.IP, c.config.Port))
	if err != nil {
		return compilerConn{}, err
	}

	rpcConn := rpc.NewConn(rpc.NewStreamTransport(netConn), nil)
	log.Info("Successfully connected to compiler service")

	return compilerConn{
		transport: netConn,
		rpc:       rpcConn,
	}, nil
}

func newCompiler(
	config CompilerConfig,
) (CompilerManager, error) {
	// Transform the artifact path into an absolute path.
	abs, err := filepath.Abs(config.ArtifactPath)
	abs = filepath.Clean(abs)
	if err != nil {
		return CompilerManager{}, fmt.Errorf("resolve artifact path: %s", err.Error())
	}
	config.ArtifactPath = abs

	// Create artifact directory if not exists.
	if err := os.MkdirAll(config.ArtifactPath, defaultFilePermissions); err != nil {
		errMsg := fmt.Sprintf("could not create Wasm artifact path: %s", err.Error())
		log.Error(errMsg)
		return CompilerManager{}, errors.New(errMsg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	compiler := CompilerManager{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	// Test if we can connect to the compiler.
	conn, err := compiler.connect()
	if err != nil {
		return CompilerManager{}, err
	}
	defer conn.rpc.Close()

	log.Infof("Initialized compiler service, local cache at `%s`", config.ArtifactPath)
	return compiler, nil
}

func (c CompilerManager) artifactPath(hash string) string {
	filePath := path.Join(c.config.ArtifactPath, fmt.Sprintf("%s.wasm", hash))
	return filePath
}

func (c *CompilerManager) getCached(hash string) ([]byte, error) {
	filePath := c.artifactPath(hash)

	file, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debugf("[compiler] Could not load cache from `%s`: file does not exist", filePath)
			return nil, nil
		}

		log.Errorf("Could not load cached Wasm artifact from compiler: %s", err.Error())
		return nil, err
	}

	log.Debugf("[compiler] Loaded cached artifact `%s`", filePath)

	if len(file) == 0 {
		log.Warnf("Tried to read empty Wasm cache file, removing file")
		if err := os.Remove(filePath); err != nil {
			return nil, fmt.Errorf("whilst removing empty cache file")
		}
		return nil, nil
	}

	return file, nil
}

func (c *CompilerManager) writeCached(hash string, artifact []byte) error {
	filePath := c.artifactPath(hash)
	if err := os.WriteFile(filePath, artifact, defaultFilePermissions); err != nil {
		log.Errorf("Could not write Wasm artifact cache from compiler: %s", err.Error())
		return err
	}
	log.Debugf("[compiler] Created artifact cache `%s`", filePath)
	return nil
}

func (c CompilerManager) hashInput(code string, language compiler.Language) string {
	buf := []byte(code)
	buf = append(buf, []byte(language.String())...)

	idBinary := sha256.Sum256(buf)
	return hex.EncodeToString(idBinary[0:])
}

func parseLanguage(from JobProgrammingLanguage) compiler.Language {
	switch from {
	case RustLanguage:
		return compiler.Language_rust
	case CLanguage:
		return compiler.Language_c
	default:
		panic("parseLanguage(): illegal language encountered")
	}
}

type CompileArtifact struct {
	Wasm []byte
	Hash string
}

func (c *CompilerManager) Compile(
	language JobProgrammingLanguage,
	programSrc string,
) (artifact CompileArtifact, comp CompilationError, system error) {
	parsedLanguage := parseLanguage(language)
	hash := c.hashInput(programSrc, parsedLanguage)

	// Check if there is already a cached artifact.
	cachedBytes, err := c.getCached(hash)
	if err != nil {
		return artifact, nil, err
	}

	// Success, there is already a cached version available.
	if cachedBytes != nil {
		return CompileArtifact{
			Wasm: cachedBytes,
			Hash: hash,
		}, nil, nil
	}

	conn, err := c.connect()
	if err != nil {
		return artifact, nil, err
	}
	defer conn.rpc.Close()

	rpcCompiler := compiler.Compiler(conn.rpc.Bootstrap(c.ctx))
	f, release := rpcCompiler.Compile(c.ctx, func(c compiler.Compiler_compile_Params) error {
		c.SetLanguage(parsedLanguage)
		return c.SetProgramSrc(programSrc)
	})
	defer release()
	defer rpcCompiler.Release()

	res, err := f.Struct()
	if err != nil {
		return artifact, nil, err
	}

	r, err := res.Response()
	if err != nil {
		return artifact, nil, err
	}

	switch r.Which() {
	case compiler.CompilerResponse_Which_fileContent:
		file, err := r.FileContent()
		if err != nil {
			log.Errorf("Could not parse file contents in compiler: %s", err.Error())
			return artifact, nil, err
		}

		if err := c.writeCached(hash, file); err != nil {
			return artifact, nil, err
		}

		fileBuf := make([]byte, len(file))
		copy(fileBuf, file)

		return CompileArtifact{
			Wasm: fileBuf,
			Hash: hash,
		}, nil, nil
	case compiler.CompilerResponse_Which_systemError:
		fs, err := r.SystemError()
		if err != nil {
			log.Errorf("Could not parse system error in compiler: %s", err.Error())
			return artifact, nil, err
		}

		return artifact, nil, errors.New(fs)
	case compiler.CompilerResponse_Which_compilerError:
		cpE, err := r.CompilerError()
		if err != nil {
			log.Errorf("Could not parse compile error in compiler: %s", err.Error())
			return artifact, nil, err
		}

		return artifact, &cpE, nil
	default:
		panic("Unreachable: a new type of compiler reply occurred")
	}
}
