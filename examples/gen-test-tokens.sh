#!/bin/sh

if ! which jwt >/dev/null; then
	echo "you must install jwt tool from https://github.com/mike-engel/jwt-cli"
	exit 1
fi

echo "token for Alice"
jwt encode --secret secret --iss pki.example.com -P email=alice@example.com --aud minibridge

echo
echo "token for Bob"
jwt encode --secret secret --iss pki.example.com -P email=bob@example.com --aud minibridge

echo
echo "Token for Eve"
jwt encode --secret "not-secret" --iss pki.example.com -P email=eve@example.com --aud minibridge
