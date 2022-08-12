#!/usr/bin/env bash

test_description="Test HTTP Gateway _redirects support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

## ============================================================================
## Test _redirects file support
## ============================================================================

# Import test case
# Run `ipfs cat /ipfs/$REDIRECTS_DIR_CID/_redirects` to see sample _redirects file
test_expect_success "Add the _redirects file test directory" '
  ipfs dag import ../t0109-gateway-web-_redirects-data/redirects.car
'
CAR_ROOT_CID=QmfHFheaikRRB6ap7AdL4FHBkyHPhPBDX7fS25rMzYhLuW

REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/examples | cut -d "/" -f3)
REDIRECTS_DIR_HOSTNAME="${REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/redirect-one redirects with default of 301, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/301-redirect-one redirects with 301, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/301-redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/302-redirect-two redirects with 302, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/302-redirect-two" > response &&
  test_should_contain "two.html" response &&
  test_should_contain "302 Found" response &&
  test_should_contain "Location:" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/200-index returns 200, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/200-index" > response &&
  test_should_contain "my index" response &&
  test_should_contain "200 OK" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/posts/:year/:month/:day/:title redirects with 301 and placeholders, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/posts/2022/01/01/hello-world" > response &&
  test_should_contain "/articles/2022/01/01/hello-world" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/splat/one.html redirects with 301 and splat placeholder, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/splat/one.html" > response &&
  test_should_contain "/redirected-splat/one.html" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/en/has-no-redirects-entry returns custom 404, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/not-found/has-no-redirects-entry" > response &&
  test_should_contain "404 Not Found" response &&
  test_should_contain "my 404" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/catch-all returns 200, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/catch-all" > response &&
  test_should_contain "200 OK" response &&
  test_should_contain "my index" response
'

test_expect_success "request for http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one returns 404, no _redirects since no origin isolation" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one" > response &&
  test_should_contain "404 Not Found" response &&
  test_should_not_contain "my 404" response
'

# With CRLF line terminator
NEWLINE_REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/newlines | cut -d "/" -f3)
NEWLINE_REDIRECTS_DIR_HOSTNAME="${NEWLINE_REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "newline: _redirects has CRLF line terminators" '
  ipfs cat /ipfs/$NEWLINE_REDIRECTS_DIR_CID/_redirects | file - > response &&
  test_should_contain "with CRLF line terminators" response
'

test_expect_success "newline: request for $NEWLINE_REDIRECTS_DIR_HOSTNAME/redirect-one redirects with default of 301, per _redirects file" '
  curl -sD - --resolve $NEWLINE_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$NEWLINE_REDIRECTS_DIR_HOSTNAME/redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

# Good codes
GOOD_REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/good-codes | cut -d "/" -f3)
GOOD_REDIRECTS_DIR_HOSTNAME="${GOOD_REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "good codes: request for $GOOD_REDIRECTS_DIR_HOSTNAME/redirect-one redirects with default of 301, per _redirects file" '
  curl -sD - --resolve $GOOD_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$GOOD_REDIRECTS_DIR_HOSTNAME/a301" > response &&
  test_should_contain "b301" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

# Bad codes
BAD_REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/bad-codes | cut -d "/" -f3)
BAD_REDIRECTS_DIR_HOSTNAME="${BAD_REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

# if accessing a path that doesn't exist, read _redirects and fail parsing, and return error
test_expect_success "bad codes: request for $BAD_REDIRECTS_DIR_HOSTNAME/not-found returns error about bad code" '
  curl -sD - --resolve $BAD_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$BAD_REDIRECTS_DIR_HOSTNAME/not-found" > response &&
  test_should_contain "500" response &&
  test_should_contain "unsupported redirect status" response
'

# if accessing a path that does exist, don't read _redirects and therefore don't fail parsing
test_expect_success "bad codes: request for $BAD_REDIRECTS_DIR_HOSTNAME/found.html doesn't return error about bad code" '
  curl -sD - --resolve $BAD_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$BAD_REDIRECTS_DIR_HOSTNAME/found.html" > response &&
  test_should_contain "200" response &&
  test_should_contain "my found" response &&
  test_should_not_contain "unsupported redirect status" response
'

# Invalid file, containing "hello"
INVALID_REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/invalid | cut -d "/" -f3)
INVALID_REDIRECTS_DIR_HOSTNAME="${INVALID_REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

# if accessing a path that doesn't exist, read _redirects and fail parsing, and return error
test_expect_success "invalid file: request for $INVALID_REDIRECTS_DIR_HOSTNAME/not-found returns error about invalid redirects file" '
  curl -sD - --resolve $INVALID_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$INVALID_REDIRECTS_DIR_HOSTNAME/not-found" > response &&
  test_should_contain "500" response &&
  test_should_contain "could not parse _redirects:" response
'

# Invalid file, containing forced redirect
INVALID_REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/forced | cut -d "/" -f3)
INVALID_REDIRECTS_DIR_HOSTNAME="${INVALID_REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

# if accessing a path that doesn't exist, read _redirects and fail parsing, and return error
test_expect_success "invalid file: request for $INVALID_REDIRECTS_DIR_HOSTNAME/not-found returns error about invalid redirects file" '
  curl -sD - --resolve $INVALID_REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$INVALID_REDIRECTS_DIR_HOSTNAME/not-found" > response &&
  test_should_contain "500" response &&
  test_should_contain "could not parse _redirects:" response &&
  test_should_contain "forced redirects (or \"shadowing\") are not supported by IPFS gateways" response
'

test_kill_ipfs_daemon

# disable wildcard DNSLink gateway
# and enable it on specific DNSLink hostname
ipfs config --json Gateway.NoDNSLink true && \
ipfs config --json Gateway.PublicGateways '{
  "dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  },
  "dnslink-disabled-on-fqdn.example.com": {
    "NoDNSLink": true,
    "UseSubdomains": false,
    "Paths": []
  }
}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
# REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/examples | cut -d "/" -f3)
DNSLINK_FQDN="dnslink-enabled-on-fqdn.example.org"
NO_DNSLINK_FQDN="dnslink-disabled-on-fqdn.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$REDIRECTS_DIR_CID"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$REDIRECTS_DIR_CID\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN/_redirects > result &&
  test_should_contain \"index.html\" result
"

test_expect_success "request for $DNSLINK_FQDN/redirect-one redirects with default of 301, per _redirects file" '
  curl -sD - --resolve $DNSLINK_FQDN:$GWAY_PORT:127.0.0.1 "http://$DNSLINK_FQDN:$GWAY_PORT/redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response &&
  test_should_contain "Location:" response
'

test_expect_success "request for $NO_DNSLINK_FQDN/redirect-one does not redirect, since DNSLink is disabled" '
  curl -sD - --resolve $NO_DNSLINK_FQDN:$GWAY_PORT:127.0.0.1 "http://$NO_DNSLINK_FQDN:$GWAY_PORT/redirect-one" > response &&
  test_should_not_contain "one.html" response &&
  test_should_not_contain "301 Moved Permanently" response &&
  test_should_not_contain "Location:" response
'

test_kill_ipfs_daemon

test_done
