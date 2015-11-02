# go-imap

[![Build Status](https://travis-ci.org/paulrosania/go-imap.svg?branch=master)](https://travis-ci.org/paulrosania/go-imap)

go-imap is a *server-side* IMAP parser.

IMAP's wire protocol is gnarly. go-imap does the parsing for you, so you can
focus on the server logic itself. (It doesn't implement the IMAP commands
itself, that's up to you.)

## Caveats

go-imap is reasonably complete, and I am using it actively, but it is *not*
production-ready yet. In particular:

* The majority of the library lacks test coverage
* `BODYSTRUCTURE` marshaling is not implemented
* Robustness to errors and edge cases varies

## Installation

    go get github.com/paulrosania/go-imap

## Documentation

Full API documentation is available here:

[https://godoc.org/github.com/paulrosania/go-imap](https://godoc.org/github.com/paulrosania/go-imap)

## Contributing

1. Fork the project
2. Make your changes
2. Run tests (`go test`)
3. Send a pull request!

If you’re making a big change, please open an issue first, so we can discuss.

## License

MIT
