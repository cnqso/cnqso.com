# cnqso.com

This is my personal website. I built it using Go's net/http library because I wanted more freedom than I had with other frameworks.

I try to keep things simple, fast, and portable. Mostly I want iteration to be fast and development to be fun.

The goal is to have one place where I can host all my web dev projects, forever.

## Deployment

Production pulls from GitHub instead of accepting a push from GitHub Actions.
After the one-time installation, every push to `main` is deployed within about a
minute:

```sh
sudo ./scripts/install-autodeploy.sh
```

The installer derives the repository path and owner, links the systemd units
shipped in `deploy/systemd/`, and starts the first deployment. The timer only
fast-forwards a clean `main` checkout. A failed build or health check is retried
on the next run, and the currently running container is left alone when a build
fails.

Useful commands:

```sh
systemctl list-timers cnqso-web-deploy.timer
journalctl -u cnqso-web-deploy.service -f
sudo systemctl start cnqso-web-deploy.service
```





### TODO
* Get sokodle running on here
    * Port all nextjs endpoints to go
    * Switch to sqlite
    * Restructure in a not-vibe-coded and extensible way
    * Make like 14 levels
    * Post it on reddit
### Backburner
* Update all links
* Automate npm build steps for submodules
    * Get off npm??
* Bypass wordledata repo for reverseWordleSolver
* Convert spirals to ts
* Get sanity CMS running here
    * Maybe abandon it. Latency is an aesthetic issue
    * Remove all lame blog articles
* Get rid of my github.io site
