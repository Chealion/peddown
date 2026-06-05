#! /bin/sh

sleep 5

./peddown load
sleep 5
./peddown process

# Add a 60 second delay in case we need to ssh into the machine
sleep 60
