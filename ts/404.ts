const audio = document.getElementById("tropicalMusic") as HTMLAudioElement;
let isPlaying = false;

audio.volume = 0.3;

function toggleMusic() {
  if (isPlaying) {
    audio.pause();
    isPlaying = false;
  } else {
    audio
      .play()
      .then(() => {
        isPlaying = true;
      })
      .catch((err) => {
        console.log("Autoplay prevented:", err);
      });
  }
}

document.addEventListener(
  "click",
  function () {
    if (!isPlaying) {
      audio
        .play()
        .then(() => {
          isPlaying = true;
        })
        .catch((err) => {
          console.log("Could not start music:", err);
        });
    }
  },
  { once: true },
);

function createParticle() {
  const particle = document.createElement("div");
  particle.style.position = "fixed";
  particle.style.width = "6px";
  particle.style.height = "6px";

  const colors = [
    "rgba(255, 255, 255, 0.8)",
    "rgba(255, 255, 0, 0.7)",
    "rgba(0, 255, 255, 0.7)",
    "rgba(255, 192, 203, 0.7)",
    "rgba(144, 238, 144, 0.7)",
  ];
  particle.style.background = colors[Math.floor(Math.random() * colors.length)];
  particle.style.borderRadius = "50%";
  particle.style.pointerEvents = "none";
  particle.style.zIndex = "0";

  const size = Math.random() * 4 + 4;
  particle.style.width = size + "px";
  particle.style.height = size + "px";

  particle.style.left = Math.random() * 100 + "vw";
  particle.style.top = "100vh";

  document.body.appendChild(particle);

  const animation = particle.animate(
    [
      { transform: "translateY(0) rotate(0deg)", opacity: 0.8 },
      { transform: "translateY(-100vh) rotate(360deg)", opacity: 0 },
    ],
    {
      duration: Math.random() * 4000 + 3000,
      easing: "linear",
    },
  );

  animation.onfinish = () => {
    particle.remove();
  };
}

setInterval(createParticle, 50);
