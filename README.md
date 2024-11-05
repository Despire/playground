# tinytorrent

Tinytorrent is a dead simple implementation of the bittorrent v1.0 specification as specified at:

1. https://wiki.theory.org/BitTorrentSpecification
2. https://www.bittorrent.org/beps/bep_0003.html


The client functions as both a downloader and an uploader, employing an optimistic unchoking strategy with connected leechers. However, it is constrained by static download and upload rates, which do not fully utilize the available bandwidth or accommodate the increasing TCP window size between connected peers,
as its meant to be a toy implementation.

The client can handle both single file and multi file torrents.

NOTE: only a small handful free non-copyrighted has been tested, so there may be cases which are not handled. Magnet links are also not supported.

# Example

Below is an example of using the client to download the latest non-copyrighted [debian.iso](./torrent/test_data/debian.torrent), 
which can be found at https://www.debian.org/CD/torrent-cd/.  The client on top is acting as both a Seeder and Leecher of the torrent 
and is connected to other peers in the  swarm for the torrent file. The client on the bottom acts only as a leecher and is connected 
to the other client  on a private network downloading pieces as the Seeder makes them available.

[![asciicast](https://asciinema.org/a/HO1qUu1tYe2IKxytWTc509p25.svg)](https://asciinema.org/a/HO1qUu1tYe2IKxytWTc509p25)

