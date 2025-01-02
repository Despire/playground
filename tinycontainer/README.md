# TinyContainer

TinyContainer is a minimal toy implementation of what is know of a linux container.

# How to run

Make sure you run these in a separate VM that won't affect your environmnet.

1. Clone repository

2. Run `make` to generate the directory structure for the underlying overlay filesystem

3. Download the ubuntu docker image `docker pull ubuntu`

4. Move the downloaded layer of the ubuntu image which can be found under `/var/lib/docker/overlay2/<some-hash>/diff/` and move its contents to the `./overlay/image` directory

    `cp -r /var/lib/docker/overlay2/<some-hash>/diff/* ./overlay/image`

5. Build the program with `go build .`

6. Run the program in `sudo` mode, i.e. execute `sudo -i` type password and run the program.

7. If everything went ok, you should have a bash terminal running in the created tiny "container" it has very limited functionality but you should be able to resolve some hostname with `getent hosts www.google.com`
