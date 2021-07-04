This repo provides a spike implementation of `validate-rosetta`, a
performance-oriented validator for [Rosetta API] implementations.

To build:

```
go build -o validate-rosetta main.go
```

To validate a Data API implementation, modify `config.json` as needed, and then
run:

```
./validate-rosetta data config.json
```


[Rosetta API]: https://www.rosetta-api.org/
