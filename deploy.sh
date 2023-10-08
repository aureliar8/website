##!/usr/bin/env bash

hugo && rsync -avz --delete public/ ubuntu@aureliar.net:/var/www/html
