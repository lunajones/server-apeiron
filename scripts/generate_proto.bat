@echo off
setlocal
set ROOT=%~dp0..
cd /d "%ROOT%"

protoc ^
  --go_out=. --go_opt=paths=source_relative ^
  --go-grpc_out=. --go-grpc_opt=paths=source_relative ^
  proto/apeiron/game/v1/game.proto

if errorlevel 1 exit /b %errorlevel%

if not exist gen\apeiron\game\v1 mkdir gen\apeiron\game\v1
move /Y proto\apeiron\game\v1\game.pb.go gen\apeiron\game\v1\game.pb.go >nul
move /Y proto\apeiron\game\v1\game_grpc.pb.go gen\apeiron\game\v1\game_grpc.pb.go >nul
