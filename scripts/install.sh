#!/bin/bash

mkdir linker-keeper
cd linker-keeper/
curl -LO https://github.com/linker-bot/linker-keeper/releases/download/v0.0.1/linker-keeper_0.0.1_linux_arm64.tar.gz
tar zxvf linker-keeper_0.0.1_linux_arm64.tar.gz

sudo curl -L https://raw.githubusercontent.com/linker-bot/linker-keeper/refs/heads/main/scripts/systemd/linker-keeper.service -o /etc/systemd/system/linker-keeper.service
sudo mkdir -p /etc/linker-keeper/
sudo curl -L https://raw.githubusercontent.com/linker-bot/linker-keeper/refs/heads/main/scripts/config/keeper.yaml -o /etc/linker-keeper/keeper.yaml

sudo mkdir -p /opt/linker-keeper/
sudo cp ./linker-keeper /opt/linker-keeper/
sudo systemctl daemon-reload
sudo systemctl start linker-keeper.service
sudo systemctl status linker-keeper.service
