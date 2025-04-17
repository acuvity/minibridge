#!/bin/sh

if ! which jwt >/dev/null; then
	echo "you must install jwt tool from https://github.com/mike-engel/jwt-cli"
	exit 1
fi

echo "token for Alice"
jwt encode --secret secret --iss pki.example.com -P email=alice@example.com --aud minibridge

echo
echo "token for Bob (will not be able to call printEnv in the example)"
jwt encode --secret secret --iss pki.example.com -P email=bob@example.com --aud minibridge

echo
echo "Token with invalid signature"
jwt encode --secret not-secret --iss pki.example.com random.issuer -P email=eve@example.com --aud minibridge
