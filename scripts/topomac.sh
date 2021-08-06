#!/bin/bash
# this script is used to generate random OUI for base mac entry of an SR Linux topology yaml file

# template_path="/tmp/topo-template.yml"
template_path="/root/srl-kne-operator/scripts/test-topo.yml"
final_path="/tmp/topology.yml"

# generate random bytes
b1=$(printf "%02X" $(shuf -i 0-255 -n1))
b2=$(printf "%02X" $(shuf -i 0-255 -n1))
mac_portion=$b1:$b2

cp $template_path $final_path

sed -i s/__RANDMAC__/$mac_portion/g $final_path