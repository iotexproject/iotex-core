# logger package
Here we use zerolog for logging (https://github.com/rs/zerolog). zerolog is a json based logger that is lightning fast and efficient in production code. See its benchmark for reference.
Json based logger is good for us to collect all logs and analysis the errors and preformace easily with json parsers. 

Usage:
```
import "logger"
logger.Print("your message")
logger.Debug().Str("StrKey", "StrValue").Float("FloatKey", float_number).Msg("Your msg");
```

There are different levels of logging in this logger
```
logger.Debug()
logger.Info()
logger.Warn()
logger.Error()
logger.Fatal()
logger.Panic()
```
Starting from Error level, you can use a Err() func to put your Err return. Example
```
logger.Error().Err(err).Msg("something is wrong!")
```
A ```Msg()``` func call is required to send out the the msg.

For more advanced features, please refer to https://github.com/rs/zerolog
