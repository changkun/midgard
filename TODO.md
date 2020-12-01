
## TODO

- [x] installation script for daemon process
- [x] authenticated gRPC calls: no need because rpc are served from local
- [x] basic auth with maximum failure control
- [x] websocket clipboard push registration/notification
- [x] iOS shortcut for clipboard data fetching
- [x] config initialization, both for client and server (can we use init for daemon/server installation?)
- [x] news page
- [ ] list all daemons, stream shell commands
- [ ] clipboard history
- [ ] VCS backup
- [ ] Better clipboard listener, implement X11 convension
- [ ] register keyboard hotkey
- [ ] UPDATE/DELETE existing resource
- [ ] Search function?
- [ ] list folder tree
- [ ] code2img, img2text

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