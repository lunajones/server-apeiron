{
    "name": "typescript",
    "author": "Microsoft Corp.",
    "homepage": "https://www.typescriptlang.org/",
    "version": "6.0.3",
    "license": "Apache-2.0",
    "description": "TypeScript is a language for application scale JavaScript development",
    "keywords": [
        "TypeScript",
        "Microsoft",
        "compiler",
        "language",
        "javascript"
    ],
    "bugs": {
        "url": "https://github.com/microsoft/TypeScript/issues"
    },
    "repository": {
        "type": "git",
        "url": "https://github.com/microsoft/TypeScript.git"
    },
    "main": "./lib/typescript.js",
    "typings": "./lib/typescript.d.ts",
    "bin": {
        "tsc": "./bin/tsc",
        "tsserver": "./bin/tsserver"
    },
    "engines": {
        "node": ">=14.17"
    },
    "files": [
        "bin",
        "lib",
        "!lib/enu",
        "LICENSE.txt",
        "README.md",
        "SECURITY.md",
        "ThirdPartyNoticeText.txt",
        "!**/.gitattributes"
    ],
    "devDependencies": {
        "@dprint/formatter": "^0.4.1",
        "@dprint/typescript": "0.93.4",
        "@esfx/canceltoken": "^1.0.0",
        "@eslint/js": "^10.0.1",
      