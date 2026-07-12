# cconv
## Currency converter using official Danish exchange rates
It will convert to and from currencies using Danmarks Nationalbank's official exchange rates.

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
### Arguments
The quickest way to convert is with positional arguments:
```bash
cconv [amount] [from] [to...]
```
A leading number is the amount; otherwise the first word is the `from` currency.
`amount` defaults to `1`, `from` defaults to `DKK`, and `to` defaults to all
currencies.
```bash
cconv 100 dkk eur       # 100 DKK -> EUR
cconv 100 dkk eur gbp   # 100 DKK -> EUR and GBP
cconv dkk eur           # 1 DKK -> EUR (amount omitted)
cconv 100 usd           # 100 USD -> all currencies
cconv                   # 1 DKK -> all currencies
```

### Flags
Flags override the positional arguments when set, and the old flag-only style
still works.
```bash
  -a, --amount float   Set amount to convert (default 1)
  -f, --from string    Choose currency to convert from (default "DKK")
  -h, --help           help for currency-converter
      --no-cache       Bypass the local cache and fetch fresh rates
  -t, --to strings     Choose currency to convert to (eg. eur,dkk). Default is all.
  -v, --verbose        Enable verbose mode
      --version        version for currency-converter
```

### Caching
Nationalbanken publishes rates once per business day (around 16:00 CET), so
fetched rates are cached under your user cache directory (e.g.
`~/.cache/cconv/rates.xml`). Cached rates from the current day are reused
directly; older ones are re-checked periodically in case newer rates have been
published. Use `--no-cache` to force a fresh fetch.
