
## TODO

- [ ] installation script for daemon process
- [ ] register keyboard hotkey
- [x] authenticated gRPC calls: no need because rpc are served from local
- [x] basic auth with maximum failure control
- [ ] Better clipboard listener, implement X11 convension
- [x] websocket clipboard push registration/notification
- [ ] UPDATE/DELETE existing resource
- [ ] Search function?
- [x] iOS shortcut for clipboard data fetching
- [ ] VCS backup
- [ ] list folder tree
- [x] config initialization, both for client and server (can we use init for daemon/server installation?)

```
location /midgard {
    proxy_pass          http://0.0.0.0:8080;
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

    proxy_read_timeout 1800;
    proxy_connect_timeout 1800;
}
```