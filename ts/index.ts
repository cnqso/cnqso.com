let tauntIndex = 0;
const tauntMessages = [
  "The door is locked",
  "It's jammed",
  "It's stuck closed. Must be locked from the other side",
  "You think you hear rustling inside",
  "You don't have the key",
  "Huh, locked",
  "You cannot enter my secret door",
  "My door is locked shut",
  "You cannot enter my secret door",
  "You cannot pass my mysterious gate",
  "You will not traverse my door",
  "'I wonder what's inside?'",
  "You cannot pass through this secret door",
  "You cannot open my door",
  "It won't give way",
  "My door is too strong and too locked",
];
document.querySelector(".bokce img").addEventListener("click", function () {
  alert(tauntMessages[tauntIndex]);
  tauntIndex = (tauntIndex + 1) % tauntMessages.length;
});
