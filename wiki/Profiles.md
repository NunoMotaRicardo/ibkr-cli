# Profiles: more than one IBKR login

Use a profile for each IBKR **login** you want to reach from the same machine, for example a live and a paper-trading login. Each profile has its own credentials and its own tool cache.

## One profile per login

One IBKR authorization covers one account, and IBKR keeps one consent per login. Each profile registers under its own name (`ibkr-cli (paper)`), which keeps consents from *different logins* apart.

It does **not** make two accounts of the *same* login independent. If a single login owns accounts A and B, connecting a profile for B replaces the consent of the profile for A, which then fails with `invalid_grant` ("Invalid authorization status"). To switch between accounts of one login, use one profile and reconnect (`ibkr --profile <name> connect`), picking the other account on IBKR's consent page.

## Set up

```
ibkr connect                       # your existing login stays the "default" profile
ibkr --profile paper connect       # log in with the paper login in the browser
ibkr profiles                      # default: connected, paper: connected
```

Names: 1-32 lowercase letters, digits, `-` or `_`.

## Use

```
ibkr --profile paper positions
export IBKR_PROFILE=paper          # for a whole shell session
```

Once more than one profile exists, every command needs `--profile` or `IBKR_PROFILE`; otherwise it stops with exit code 2 and lists the profiles. This is deliberate: a command can never silently act on the wrong account. `version`, `help`, `completion` and `profiles` never need one.

## Remove

`ibkr --profile paper logout` revokes and deletes that profile only.

## Where things live

| What | Default profile | Named profile |
|---|---|---|
| Keyring user (service `ibkr-cli`) | `default` | `profile/<name>` |
| Fallback file | `<config>/ibkr-cli/credentials.json` | `<config>/ibkr-cli/profiles/<name>/credentials.json` |
| Tool cache | `<cache>/ibkr-cli/` | `<cache>/ibkr-cli/profiles/<name>/` |
| Profile list | `<config>/ibkr-cli/profiles.json` | |

`IBKR_ACCESS_TOKEN` overrides the stored token for any profile and needs no profile selection.
