module github.com/reef-runtime/reef/reef_manager

go 1.21.10

require (
	capnproto.org/go/capnp/v3 v3.0.0-alpha-29
	github.com/Masterminds/squirrel v1.5.4
	github.com/davecgh/go-spew v1.1.1
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-migrate/migrate/v4 v4.17.1
	github.com/gorilla/websocket v1.5.1
	github.com/ilyakaznacheev/cleanenv v1.5.0
	github.com/lib/pq v1.10.9
	github.com/reef-runtime/reef/reef_protocol_compiler v0.0.0-00010101000000-000000000000
	github.com/reef-runtime/reef/reef_protocol_node v0.0.0-00010101000000-000000000000
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.3
)

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/gobuffalo/here v0.6.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/markbates/pkger v0.17.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.20.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
	zenhack.net/go/util v0.0.0-20230414204917-531d38494cf5 // indirect
)

replace github.com/reef-runtime/reef/reef_protocol_node => ../reef_protocol/node/go/

replace github.com/reef-runtime/reef/reef_protocol_compiler => ../reef_protocol/compiler/go/
