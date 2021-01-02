# Midgard Installation

## Architecture

Before start installing/using midgard, it is necessary to
understand how midgard works. The midgard service contains three parts:

- CLI
- Daemon
- Server

A user uses midgard CLI communicate with the midgard daemon on local device,
and the daemon process talks to the midgard server for synchornization/allocation
between devices.

```
                            HTTPS
Mobile <-----------------------------------------------┐
                                                       |
CLI    <-------> daemon <-----┐  Secure Websocket      v     HTTPS
          RPC                 ├--------------------> server <------> public
CLI    <-------> daemon <-----┘
```


## Dependencies

macOS:

```
$ xcode-select --install
```

Linux:

```
$ sudo apt install -y git xclip libx11-dev
```

## Binary Build

```
$ git clone https://github.com/changkun/midgard
$ make
$ ln "$(pwd)/mg" /usr/local/bin/mg
$ mg help
midgard is a universal clipboard service developed by Changkun Ou.
See https://changkun.de/s/midgard for more details.

Usage:
  mg [command]
```

## Docker Build

Docker build requires you to setup environment variable `SSH_KEY_PATH`
that points to a RSA private key file, for example:

```
$ echo $SSH_KEY_PATH
~/.ssh/id_rsa
```

```
$ make build
```

## Configuration

To configure midgard settings:

- in a configuration file, see [config.yml](../config.yml) for more details.
- Or use environment variable `MIDGARD_CONF=/path/to/your/config.yml` to change the location of [config.yml](../config.yml).

## Midgard Server

Docker:

```
$ make up
```

> Hint: You need understand how [docker-compose](../docker-compose.yml) works.

Native:

```sh
$ mg server
```

## Midgard Daemon

`midgard` daemon process **runs on your local machine**
(automatic start when machine boots):

```sh
$ mg daemon install
$ mg daemon start
$ mg daemon stop
$ mg daemon uninstall
```

> Linux requires `sudo`

or

```sh
$ mg daemon run
```

## Reverse Proxy

If midgard is deployed behind an nginx server, then the following
configuration could help:

```
location /midgard {
    proxy_pass          http://0.0.0.0:80;
    proxy_set_header    Host             $host;
    proxy_set_header    X-Real-IP        $remote_addr;
    proxy_set_header    X-Forwarded-For  $proxy_add_x_forwarded_for;
    proxy_set_header    X-Client-Verify  SUCCESS;
    proxy_set_header    X-Client-DN      $ssl_client_s_dn;
    proxy_set_header    X-SSL-Subject    $ssl_client_s_dn;
    proxy_set_header    X-SSL-Issuer     $ssl_client_i_dn;

    # websocket support
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    client_max_body_size 2M;
}
```

If you use traefik, then the following configuration could help:

### Static configuration

```yaml
entryPoints:
  web:
    address: :80
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: :443

certificatesResolvers:
  changkunResolver:
    acme:
      email: your@email.com
      storage: /path/to/your/acme.json
      httpChallenge:
        entryPoint: web
```

### Dynamic configuration

```yaml
http:
  routers:
    to-midgard:
      rule: "Host(`example.com`)&&PathPrefix(`/midgard`)"
      tls:
        certResolver: yourCertResolver
      service: midgard
  services:
    midgard:
      loadBalancer:
        servers:
        - url: http://midgard
```


## License

Copyright 2020 [Changkun Ou](https://changkun.de). All rights reserved.