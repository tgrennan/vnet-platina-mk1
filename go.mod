module github.com/platinasystems/vnet-platina-mk1

require (
	github.com/garyburd/redigo v1.6.0
	github.com/platinasystems/atsock v1.1.0
	github.com/platinasystems/buildid v1.0.0
	github.com/platinasystems/buildinfo v1.1.0
	github.com/platinasystems/dbg v1.1.0
	github.com/platinasystems/elib v1.3.2
	github.com/platinasystems/fe1 v1.3.11
	github.com/platinasystems/firmware-fe1a v1.1.0
	github.com/platinasystems/i2c v1.1.0
	github.com/platinasystems/log v1.1.0
	github.com/platinasystems/redis v1.2.0
	github.com/platinasystems/vnet v1.4.6
	github.com/platinasystems/xeth v1.2.0
	gopkg.in/yaml.v2 v2.2.1
)

replace github.com/platinasystems/xeth => github.com/tgrennan/xeth v1.1.2-0.20190729175040-2fa4f2ee8f84

replace github.com/platinasystems/vnet => github.com/tgrennan/vnet v1.2.0-rc.1.0.20190729175317-142778231287

replace github.com/platinasystems/elib => github.com/tgrennan/elib v1.2.1-0.20190725024808-68c86557cdc4

replace github.com/platinasystems/fe1 => github.com/tgrennan/fe1 v0.0.0-20190729175534-cfc896dea7ff
