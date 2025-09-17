# Making Online Games Should Be Easy
## 2025-09-18

It's an established maxim that new indie developers should "avoid multiplayer". It's understood that building a game is hard, and the added complication of backend infrastructure will make it impossible. This makes sense on first blush, but the more I think about it the more I wonder if it's really true. As a fullstacker, I feel like web dev is the easiest thing in the world. It's game development and graphics programming that are truly difficult, web developers are comparably playing with legos.

Back in 2023, I started working on "Commons", a multiplayer city-building game based closely on the original SimCity. I was brand new to web development, and was feeling my way through using youtube tutorials. The frontend was built using React (animations all on an HTML canvas with useEffect), and my backend was hosted on Firebase using their awful instant-lock-in serverless framework. I spent 6 sleepless, unemployed weeks on the project and ended with something barely playable before I made the tough decision to abandon it. The bottleneck was not the netcode as you might expect, but the difficult task of tuning a simulation game to be fun. My choice to make the game multiplayer had no impact on my ability to complete it. If anything, it made the barely-playable end result less pathetic. Web development is easy and game development is hard.

The incentive to make online games is there, too. You have the "friendslop" genre, including games like Lethal Company or PEAK, where friends queue up together to complete collaborative tasks. The gameplay is typically simple and minimal, with the social element making up most of the draw. Elsewhere you have "io" games like agar.io or slither.io, where large lobbies of random players compete in a simple, wordless game. As single-player games these would be trite and boring, but as multiplayer games their relatively simple design works.

And frankly, making an online game just sounds more fun and rewarding. The game-jam game -- a simple, short, unfinished project you complete in a weekend -- would lend itself so perfectly to multiplayer. The game-jam game has hit essentially every genre besides online multiplayer. Where are the game-jam Club Penguin-likes?

What's missing for indie developers is a clear, idiomatic path forward. With the exception of certain Unity or Unreal plugins (which will lock you in and bleed you dry btw), most games are stuck building their own netcode from scratch or near-scratch. There are plenty of SaaS solutions out there to jumpstart game studios, but indie devs are left starting from scratch in the AWS console like cavemen.

For a game like agar.io, we could easily imagine how this might be structured. We could set up a load balancer to automatically route traffic to an open server. If all servers are full, we can automatically spin up a new one. The server keeps track of every entity's position, velocity, and size, and runs a simulation step every couple milliseconds. The player sends us their input (in this case, their relative mouse position) and we send back the full state of the game world. You could get the whole thing up and running in a week.

Now let's imagine we're the creator of agar.io and want to make Counter Strike 3. Could we share any of our infrastructure with agar.io? What would stay the same and what would need to change? Where would we need to split off?

Whatever can be shared between the two is "NextJS for games," my trillion dollar idea.

1. Slap together a TCP-UDP server and a client binary.
2. Add in a module that handles the game logic.
3. Develop a Godot plugin that connects with that module so that the dev can write their own game with minimal departures from their preferred IDE.
4. Find a business guy to monetize it for you.
5. Retire to a tropical island to drink mojitos and make ugly linux distros like DHH.
