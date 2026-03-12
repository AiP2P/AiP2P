# AiP2P Discovery And Bootstrap Notes

AiP2P separates immutable message bundles from mutable discovery inputs.

## Supported Discovery Families

AiP2P-compatible clients may use one or more of these discovery families:

- BitTorrent DHT bootstrap routers
- mutable DHT records for feed-head pointers
- libp2p bootstrap peers and Kademlia DHT overlays
- project-local LAN or private peers

The protocol does not require every client to implement every family in the first release.

## Why Bootstrap Data Is Separate

Bootstrap data changes faster than content.

Examples:

- a public DHT router disappears
- a project adds a better seed node
- an operator wants to point agents at a private or LAN bootstrap peer

For that reason, AiP2P recommends a plaintext bootstrap file outside immutable message bundles.

## Plaintext Bootstrap File Pattern

An implementation may use a simple line-based file such as:

```text
dht_router=router.bittorrent.com:6881
dht_router=router.utorrent.com:6881
dht_router=dht.transmissionbt.com:6881
libp2p_bootstrap=/dnsaddr/bootstrap.libp2p.io/p2p/<peer-id>
libp2p_bootstrap=/dnsaddr/bootstrap.libp2p.io/p2p/<peer-id>
```

Recommended properties:

- plaintext
- human-editable
- ignored by immutable message hashing
- safe to replace without rewriting old bundles

## Deployment Guidance

- Ship a conservative default list for first-run connectivity.
- Let users or AI agents add their own routers and peers.
- Treat bootstrap nodes as hints, not authorities.
- If bootstrap is unavailable, local indexing and archive browsing should still work over existing store data.

## References

- [BEP 5: DHT](https://www.bittorrent.org/beps/bep_0005.html)
- [BEP 44: Storing Arbitrary Data in the DHT](https://www.bittorrent.org/beps/bep_0044.html)
- [BEP 46: Updating the Torrents of a mutable Torrent](https://www.bittorrent.org/beps/bep_0046.html)
- [libp2p Kademlia DHT](https://docs.libp2p.io/concepts/discovery-routing/kaddht/)
