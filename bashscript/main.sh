#!/usr/bin/env bash

ssh_into_vm() {
  ip=$1
  # echo "ssh t1@$ip"
  # echo "sshpass -p "$password" ssh -o 'StrictHostKeyChecking no' t1@$ip 'ls'"
  # sshpass -p "$password" ssh -o 'StrictHostKeyChecking no' t1@$ip 'ls'
  echo "$ip" 
  sshpass -p "$password" scp ink-miner-new.go webapp-new.go art-app-tester2-new.go config-new.json t1@$ip:./proj1_k4c9_u1d0b_v3c0b_y3b9
}

if [ "$#" != 2 ]; then
  echo "Usage [main.sh] [p | q | t] [remote password]"
  exit
fi

env=$1
password=$2

if [ "$env" == "p" ]; then
  echo "using P environment"
  environment="p"
  
  # Now read the public ips for P
  filename="pips.pub"
  while read -u10 line
  do
    pip="$line"
    
    # TODO: Use pip to ssh
    ssh_into_vm $pip

  done 10< "$filename"

elif [ "$env" == "q" ]; then
  echo "using Q environment"
  environment="q"

  # Now read the public ips for Q
  filename="qips.pub"
  while read -u10 line
  do
    qip="$line"

    # TODO: Use qip to ssh
    ssh_into_vm $qip

  done 10< "$filename"

# THIS IS ONLY FOR TEST ENVIRONMENT
elif [ "$env" == "t" ]; then
  echo "using T environment"
  environment="t"

  # Now read the public ips for Q
  filename="tips.pub"
  while read -u10 line
  do
    tip="$line"

    ssh_into_vm $tip

  done 10< "$filename"


else 
  echo "Usage [main.sh] [p | q | t] [remote password]"
  exit
fi