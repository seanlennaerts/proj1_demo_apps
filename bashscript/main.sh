#!/usr/bin/env bash

if [ "$1" == "p" ]; then
  echo "using P environment"
  environment="p"
  
  # Now read the public ips for P
  filename="pips.pub"
  while read -r line
  do
    pip="$line"
    
    # TODO: Use pip to ssh
    echo "$pip"
    
  done < "$filename"

elif [ "$1" == "q" ]; then
  echo "using Q environment"
  environment="q"

  # Now read the public ips for Q
  filename="qips.pub"
  while read -r line
  do
    qip="$line"

    # TODO: Use qip to ssh
    echo "$qip"

  done < "$filename"

else 
  echo "Usage [main.sh] [p or q] "
fi

