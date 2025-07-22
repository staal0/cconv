# cconv
## Simple CLI currency converter for Nykredit
It will convert to and from currencies using Nykredits exchange rates.

## Installation
### Homebrew

#### Add tap
```bash
brew tap staal0/cconv https://github.com/staal0/cconv
```
#### Brew install
```bash
brew install staal0/cconv/cconv
```

### Manually
Grab the latest release [binaries](https://github.com/staal0/cconv/releases).

## Usage
### Flags
```bash
  -a, --amount float   Set amount to convert (default 1)
  -f, --from string    Choose currency to convert from (default "DKK")
  -h, --help           help for currency-converter
  -t, --to strings     Choose currency to convert to (eg. eur,dkk). Default is all.
  -v, --verbose        Enable verbose mode
      --version        version for currency-converter
```
### Example usage
```bash
cconv -a 100 -f dkk -t gbp
DKK 100
GBP 11.579
```
