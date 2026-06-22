module server-apeiron

go 1.23

require (
	db-apeiron v0.0.0
	google.golang.org/grpc v1.73.0
)

replace db-apeiron => ../db-apeiron
