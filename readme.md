# The second incarnation of the bot for Telegram
This bot accepts magnet links and checks torrent metadata to tell apart movies and TV shows. 
### Genaral info
In this version classification is made by a model trained with Fastai ULMFiT.
There are some preconditions to start using the bot.
- _**Fix the paths**_ to executables in *.service files according to your preferences.
- _**Fix the config file**_ according to your preferences. First `mv example.config.toml config.toml` then fill `users` with Telegram user IDs allowed to control the bot and set `token` to Telegram bot token in the format described in config file. Set `downloadDirectories` to point to the right places.
- Install aria2c to the system or run it in docker container. I use system-wide install on Debian managed with included here aria2.service systemd unit-file.
```
sudo cp units/aria2.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable aria2.service
sudo systemctl start aria2.service
```
- Build the code with `GOOS=linux GOARCH=amd64 go build -o n2bot cmd/*` command (don't forget to change OS and Arch to fit) and add it to systemd services too. The bot depends on aria2 and exits with error if the connectivity is lost. The n2bot.service depends on aria2.service and would try to start it. 
```
sudo cp units/n2bot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable n2bot.service
sudo systemctl start n2bot.service
```
- It is _**optional**_ to run the classification service. If you chose not to use classification the bot would ask you to manually select the download category after collecting torrent metadata. To run the classificator locally you have to install Docker and Docker Compose. It is dockerized to prevent all the Pythony mess in the system. Please be aware that the docker image is couple Gb large as it contains the whole Fastai framework with its dependancies. If you want to run it outside the container or run it on a separate server please take a look at its repository: https://bitbucket.org/illabo/torclassr. It's on Bitbucket because of Github's 100 Mb per file limit, but the trained model file is ~150 Mb.
```
cp classr/docker-compose.yml ~/classificator/
sudo cp units/classificator-docker.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable classificator-docker.service
sudo systemctl start classificator-docker.service
```
_It is highly appriciated if someone could help to export the model in TorchScript format. AFAIK TorchScipt model could be packaged to use it as a part of this project with C-Go wrappers._
### Using the bot
There are several commands you could send to the bot.
Short form | Long form | Alternative | Description
-----------|-----------|-------------|------------
`-t=`category|`--type` category|`-t:`category|Accepts the name of category as an argument. Available categories are  "movies", "series" and "general". There are also some synonims to category names: "movies" are also could be called "film", "kino"; "series" are "tv", "show"; "general" downloads are also called "all" and "common".
`-d=`directory|`--dir` directory|`-d:`directory|Creates the _subdirectory_ for download within standart directory of a category.
`-k=`GID|`--kill` GID|`-k:`GID|Stops an aria2 task by the GID provided. User is allowed only to stop the tasks they've initiated. User won't be allowed to stop other user's tasks.
`-a`|`--tellactive`|`--tell-active`|Returns the list of all active tasks with its GID (aria2 task ID), name, and % of download completeness.
||||Please note that `-t=` and `-d=` flags would only work in the same message with the magnet link.
### Additional thingies
- iOS workflow to extract a magnet link from web page to clipboard https://www.icloud.com/shortcuts/8a7da7c8c28245c993755031f05239d2. It's quite tricky to copy-paste a magnet link since iOS 13. On a long press Safari fails to preview the link and on a short press it reports that the link is broken. However with this workflow you just need to navigate to the page with a magnet on it. Once executed workflow copies the first found magnet link to clipboard. 
- First version of the bot available at https://github.com/illabo/nasbot. It was single-file-python2-spaghetti-mess on one hand and the first not fixed or stackoverflow-developed but fully written by myself project on another.
- Scripts to collect dataset and Jupyter notebook to train the model for classificator are here: https://github.com/illabo/torrent_categorizer.
- Most of the times torrent == piracy is true. Please remember that piracy is illegal and sometimes even immoral. However I beleave that it is ok to download the things someone already bought on different media. E.g. if you have a DVD it's kinda ok to download the movie from it to conviniently use on home media server. 
### Licence
<a rel="license" href="http://creativecommons.org/licenses/by-sa/4.0/"><img alt="Creative Commons BY-SA" style="border-width:0" src="https://i.creativecommons.org/l/by-sa/4.0/88x31.png" /></a><br /><a rel="license" href="http://creativecommons.org/licenses/by-sa/4.0/">Creative Commons Attribution-ShareAlike 4.0</a>