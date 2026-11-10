# Vision

**English** · [中文](VISION.zh-CN.md)

## One sentence

A Kubo panel a beginner can actually use, where an expert is not locked out of the real settings.

## Usable means a page, not a scavenger hunt

Kubo runs. The happy path is still a pile of flags and one big JSON file. People get stuck on "is the node even up?" and leave.

The panel puts the common jobs on one screen: is the node up, where is the file, what is pinned, what a content ID points at, which way is this node set up. The words should make sense. Both languages ship together. The home page does not open with every Kubo field on it.

## Like Baota or 1Panel, and only for Kubo

Baota (宝塔) and 1Panel taught a lot of people a simple habit: log into one browser page and operate the service, without first memorizing where the config file lives.

We want that habit for Kubo. We do not want their whole job list.

This repo does not install websites, create databases, manage Docker, or edit the operating system. If a request is "also handle Nginx", the answer is no. The panel is for the IPFS node.

## On this machine, or in a browser somewhere else

One UI, two seats:

- Local: you are at the machine that runs Kubo. The panel is a page on that machine.
- Remote: you open the panel in a browser on another device. You get in through our API, and only after auth.

Remote does not mean "forward Kubo's port". We are not picking a desktop shell (Electron or anything like it) in this foundation. The UI is a web app so both seats can share it later.

## The browser talks to our API

Kubo's own HTTP API, usually `127.0.0.1:5001`, stays on the loopback interface. We do not bind it to a public address, we do not document a port forward for it, and we do not put it behind a reverse proxy "just for convenience".

The browser talks to `services/manager`. That process is our API gateway. It checks the session, then calls only the Kubo operations the panel knows how to explain: start and stop, and the file actions. It is not a pass-through of the raw RPC.

Right now the manager listens on `127.0.0.1:8080` unless `MANAGER_ADDR` is set. It answers `GET /health` for itself. `GET /api/node` plus start, stop, and restart, and the files routes, need the session from [Auth](design/auth.md). Do not publish the manager until you have read that. The files routes never hand the browser Kubo's RPC address.

## Scenes first, raw config later

Most people need a setup they can say out loud:

- a quiet node on this machine
- share a folder with someone
- pin a set of content
- a read-only public gateway

Those are scenes. A scene is a named setup the panel can describe in one sentence, then write into Kubo's config. Guessing at raw keys is not the normal path.

Experts still get the raw config, logs, and peering. That is expert mode, and it comes after the common path works. It is not the home page.

## Languages from the first screen

English and 中文 ship together. `zh` means Simplified Chinese. A new string lands in both locale files in the same change. Translation is not a cleanup task we do later.

A Traditional Chinese locale can show up when someone needs it. It is another locale file, not a second product.

## One node, then more

More than one Kubo node is a real goal. It is not an early one. Get start, stop, files, pins, and scenes solid for a single node first.

Think of the manager as the thing that knows about "the node". A second node should be an addition on that idea, not a reason to throw the API away. Do not build a cluster console now.

## Where the product lives

winx.run is the front door. The code, the issues, and this roadmap live in `ipfs-panel`. There is no other "real" tree hiding somewhere else.
