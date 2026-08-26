type Clef = "treble" | "bass";

type NoteCard = {
  id: string;
  clef: Clef;
  pitch: string;
  answer: string;
};

const noteNames = ["C", "D", "E", "F", "G", "A", "B"];
const letterIndex: Record<string, number> = {
  C: 0,
  D: 1,
  E: 2,
  F: 3,
  G: 4,
  A: 5,
  B: 6,
};

const canvas = document.querySelector<HTMLCanvasElement>("#staff-canvas");
const correctCount = document.querySelector<HTMLElement>("#correct-count");
const streakCount = document.querySelector<HTMLElement>("#streak-count");
const feedback = document.querySelector<HTMLElement>("#feedback");
const answerButtons = document.querySelector<HTMLElement>("#answer-buttons");

if (!canvas || !correctCount || !streakCount || !feedback || !answerButtons) {
  throw new Error("Piano flashcard page is missing required elements.");
}

let notes: NoteCard[] = [];
let currentNote: NoteCard | null = null;
let previousNoteId: string | null = null;
let correct = 0;
let streak = 0;
let locked = false;

function diatonicIndex(pitch: string): number {
  const match = /^([A-G])(\d+)$/.exec(pitch);
  if (!match) {
    throw new Error(`Invalid pitch: ${pitch}`);
  }

  return Number(match[2]) * 7 + letterIndex[match[1]];
}

function staffReference(clef: Clef): { pitch: string; line: "top" | "bottom" } {
  if (clef === "treble") {
    return { pitch: "E4", line: "bottom" };
  }

  return { pitch: "A3", line: "top" };
}

function noteY(note: NoteCard, topLineY: number, lineSpacing: number): number {
  const step = lineSpacing / 2;
  const reference = staffReference(note.clef);
  const referenceY = reference.line === "top" ? topLineY : topLineY + lineSpacing * 4;

  return referenceY - (diatonicIndex(note.pitch) - diatonicIndex(reference.pitch)) * step;
}

function setupCanvas(): CanvasRenderingContext2D {
  const rect = canvas.getBoundingClientRect();
  const scale = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.round(rect.width * scale));
  canvas.height = Math.max(1, Math.round(rect.height * scale));

  const context = canvas.getContext("2d");
  if (!context) {
    throw new Error("Could not create canvas context.");
  }

  context.setTransform(scale, 0, 0, scale, 0, 0);
  context.clearRect(0, 0, rect.width, rect.height);
  return context;
}

function drawClef(context: CanvasRenderingContext2D, clef: Clef, x: number, centerY: number, lineSpacing: number): void {
  context.fillStyle = "#171717";
  context.font = `${lineSpacing * 3.8}px "Times New Roman", Georgia, serif`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.fillText(clef === "treble" ? "𝄞" : "𝄢", x, centerY + (clef === "treble" ? lineSpacing * 0.12 : 0));
}

function drawLedgerLines(
  context: CanvasRenderingContext2D,
  y: number,
  noteX: number,
  topLineY: number,
  bottomLineY: number,
  lineSpacing: number,
): void {
  const halfStep = lineSpacing / 2;
  const width = lineSpacing * 2.3;
  context.beginPath();

  for (let lineY = topLineY - lineSpacing; lineY >= y - halfStep; lineY -= lineSpacing) {
    context.moveTo(noteX - width / 2, lineY);
    context.lineTo(noteX + width / 2, lineY);
  }

  for (let lineY = bottomLineY + lineSpacing; lineY <= y + halfStep; lineY += lineSpacing) {
    context.moveTo(noteX - width / 2, lineY);
    context.lineTo(noteX + width / 2, lineY);
  }

  context.stroke();
}

function drawNote(context: CanvasRenderingContext2D, x: number, y: number, lineSpacing: number): void {
  context.save();
  context.translate(x, y);
  context.rotate(-0.28);
  context.beginPath();
  context.ellipse(0, 0, lineSpacing * 0.63, lineSpacing * 0.43, 0, 0, Math.PI * 2);
  context.fillStyle = "#171717";
  context.fill();
  context.restore();
}

function drawSingleStaff(
  context: CanvasRenderingContext2D,
  clef: Clef,
  topLineY: number,
  staffLeft: number,
  staffRight: number,
  lineSpacing: number,
): void {
  for (let i = 0; i < 5; i += 1) {
    const lineY = topLineY + lineSpacing * i;
    context.beginPath();
    context.moveTo(staffLeft, lineY);
    context.lineTo(staffRight, lineY);
    context.stroke();
  }

  drawClef(context, clef, staffLeft + lineSpacing * 1.15, topLineY + lineSpacing * 2, lineSpacing);
}

function drawGrandStaffBrace(
  context: CanvasRenderingContext2D,
  staffLeft: number,
  trebleTopY: number,
  bassBottomY: number,
  lineSpacing: number,
): void {
  const x = staffLeft - lineSpacing * 0.7;

  context.beginPath();
  context.moveTo(staffLeft, trebleTopY);
  context.lineTo(staffLeft, bassBottomY);
  context.stroke();

  context.save();
  context.font = `${lineSpacing * 7.5}px "Times New Roman", Georgia, serif`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.fillText("{", x, (trebleTopY + bassBottomY) / 2);
  context.restore();
}

function drawStaff(note: NoteCard): void {
  const context = setupCanvas();
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  const lineSpacing = Math.min(width / 20, height / 14);
  const staffWidth = width * 0.72;
  const staffLeft = (width - staffWidth) / 2;
  const staffRight = staffLeft + staffWidth;
  const trebleTopY = height * 0.24;
  const bassTopY = height * 0.62;
  const activeTopY = note.clef === "treble" ? trebleTopY : bassTopY;
  const activeBottomY = activeTopY + lineSpacing * 4;
  const noteX = staffLeft + staffWidth * 0.62;
  const y = noteY(note, activeTopY, lineSpacing);

  context.lineWidth = Math.max(2, lineSpacing * 0.07);
  context.strokeStyle = "#171717";

  drawSingleStaff(context, "treble", trebleTopY, staffLeft, staffRight, lineSpacing);
  drawSingleStaff(context, "bass", bassTopY, staffLeft, staffRight, lineSpacing);
  drawGrandStaffBrace(context, staffLeft, trebleTopY, bassTopY + lineSpacing * 4, lineSpacing);
  drawLedgerLines(context, y, noteX, activeTopY, activeBottomY, lineSpacing);
  drawNote(context, noteX, y, lineSpacing);
}

function chooseNextNote(): void {
  if (notes.length === 0) {
    return;
  }

  let next = notes[Math.floor(Math.random() * notes.length)];
  if (notes.length > 1) {
    while (next.id === previousNoteId) {
      next = notes[Math.floor(Math.random() * notes.length)];
    }
  }

  currentNote = next;
  previousNoteId = next.id;
  locked = false;
  feedback.textContent = "";
  feedback.className = "feedback";
  answerButtons.querySelectorAll("button").forEach((button) => {
    button.classList.remove("correct", "incorrect");
    button.removeAttribute("disabled");
  });
  drawStaff(next);
}

function updateScore(): void {
  correctCount.textContent = String(correct);
  streakCount.textContent = String(streak);
}

function handleAnswer(answer: string, button: HTMLButtonElement): void {
  if (!currentNote || locked) {
    return;
  }

  locked = true;
  const isCorrect = answer === currentNote.answer;

  if (isCorrect) {
    correct += 1;
    streak += 1;
    button.classList.add("correct");
    feedback.textContent = "Correct";
    feedback.classList.add("right");
  } else {
    streak = 0;
    button.classList.add("incorrect");
    feedback.textContent = `Nope, ${currentNote.pitch} is ${currentNote.answer}`;
    feedback.classList.add("wrong");
    const correctButton = answerButtons.querySelector<HTMLButtonElement>(`button[data-answer="${currentNote.answer}"]`);
    correctButton?.classList.add("correct");
  }

  updateScore();
  answerButtons.querySelectorAll("button").forEach((candidate) => {
    candidate.setAttribute("disabled", "true");
  });
  window.setTimeout(chooseNextNote, isCorrect ? 450 : 1000);
}

function renderAnswerButtons(): void {
  answerButtons.innerHTML = "";
  noteNames.forEach((noteName) => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = noteName;
    button.dataset.answer = noteName;
    button.addEventListener("click", () => handleAnswer(noteName, button));
    answerButtons.appendChild(button);
  });
}

async function init(): Promise<void> {
  const response = await fetch("/static/data/piano-notes.json");
  if (!response.ok) {
    throw new Error(`Could not load note data: ${response.status}`);
  }

  notes = await response.json();
  renderAnswerButtons();
  updateScore();
  chooseNextNote();
}

window.addEventListener("resize", () => {
  if (currentNote) {
    drawStaff(currentNote);
  }
});

init().catch((error) => {
  feedback.textContent = error instanceof Error ? error.message : "Could not start flashcards.";
  feedback.classList.add("wrong");
});
