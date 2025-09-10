/** @format */


type Words = string[];
type Answers = Record<number, string>;
type GlobalJsonData = { answers: Answers, words: Words };
let globalJsonData: GlobalJsonData = { answers: {}, words: [] };

export {}

async function fetchData(): Promise<GlobalJsonData> {

	const answerData = await fetchAnswers();
    const wordList = await fetchCommonWords();

	if (answerData && wordList) {
		const result = {answers: answerData, words: wordList};
        // createDownloadLink(result);
        return result;
	} else {
		console.error("JSON data not found");
		return { answers: {}, words: [] };
	}
}

async function fetchAnswers(): Promise<Answers> {
	try {
		const response = await fetch(
			"https://raw.githubusercontent.com/cnqso/wordle-data/main/answers.json"
		);
		if (!response.ok) {
			throw new Error(`HTTP error! Status: ${response.status}`);
		}
		const text = await response.text();
		const json = JSON.parse(text);
		return json;
	} catch (error) {
		console.error("Error fetching the text file:", error);
    return {};
	}
}


async function fetchCommonWords(): Promise<Words> {
	try {
		const response = await fetch(
			"https://raw.githubusercontent.com/cnqso/wordle-data/main/word_frequency.json"
		);
		if (!response.ok) {
			throw new Error(`HTTP error! Status: ${response.status}`);
		}
		const text = await response.text();
		const json = JSON.parse(text);
		return json;
	} catch (error) {
		console.error("Error fetching the text file:", error);
		return [];
	}
}

function reverseSolver(wordleString, answer, validWords) {
	// input: string of rights and wrongs, answer, validWords
	// Output 1. The 2D solution array 2. The index of the "optimal" solutions

	const wordleArray = wordleString.match(/.{1,5}/g);
	const [allPaths, checks, foundMatches] = bruteForceAllPaths(wordleArray, answer, validWords);
	let possiblePaths = allPaths[0].length;
	for (let i = 1; i < allPaths.length; i++) {
		possiblePaths = possiblePaths * allPaths[i].length;
	}
	const standards = {
		yellowNeglect: false,
		yellowRepeats: false,
		blankRepeats: false,
		allowRepeats: false,
	};

	let standardsMet = 4;
	let solution = depthWordleSearch("", allPaths, wordleArray, standards);
	if (solution === false) {
		standards.yellowNeglect = true;
		solution = depthWordleSearch("", allPaths, wordleArray, standards);
		standardsMet--;
	}
	if (solution === false) {
		standards.yellowRepeats = true;
		solution = depthWordleSearch("", allPaths, wordleArray, standards);
		standardsMet--;
	}
	if (solution === false) {
		standards.blankRepeats = true;
		solution = depthWordleSearch("", allPaths, wordleArray, standards);
		standardsMet--;
	}
	if (solution === false) {
		standards.allowRepeats = true;
		solution = depthWordleSearch("", allPaths, wordleArray, standards);
		standardsMet--;
	}
	if (solution === false) {
		console.log("No solution found");
		standardsMet--;
	}
	// try with different standards bla bla bla
	const solutionIndexes: number[] = [];
	for (let i = 0; i < solution.length; i++) {
		solutionIndexes.push(allPaths[i].indexOf(solution[i]));
	}
	solutionIndexes.push(0)
	return [allPaths, solutionIndexes, possiblePaths, checks, foundMatches, standardsMet];
}

function bruteForceAllPaths(wordleArray: string[], answer: string, validWords: Words): [Words[], number, number] {
	// input: array of 5-letter sequences, answer, validWords
	// output: array of 5 arrays of words which match the sequence
	let checks = 0;
	let foundMatches = 0;
	const allPaths: Words[] = [];
	for (let i = 0; i < wordleArray.length-1; i++) {
		const matches: Words = [];
		const sequence = wordleArray[i];

		for (let j = 0; j < validWords.length; j++) {
			if (checks % 10000 === 0) {
				console.log(`Checked ${checks} words, found ${foundMatches} matches`);
			}
			checks++;
			if (wordMatchesSequence(validWords[j], sequence, answer)) {
				foundMatches++;
				matches.push(validWords[j]);
			}
		}
		allPaths.push(matches);
	}
	allPaths.push([answer])
	console.log(`Checked ${checks} words, found ${foundMatches} matches`);

	return [allPaths, checks, foundMatches];
}

function wordMatchesSequence(word, sequence, answer) {
	for (let i = 0; i < 5; i++) {
		if (sequence[i] === "B") {
			if (answer.includes(word[i])) {
				return false;
			}
		}
		if (sequence[i] === "G") {
			if (answer[i] !== word[i]) {
				return false;
			}
		}
		if (sequence[i] === "Y") {
			if (answer[i] === word[i]) {
				return false;
			}
			if (!answer.includes(word[i])) {
				return false;
			}
		}
	}
	return true;
}

function depthWordleSearch(
	currentWord: string,
	wordNodes: Words[],
	wordleArray: string[],
	standards: { yellowNeglect: boolean, yellowRepeats: boolean, blankRepeats: boolean, allowRepeats: boolean },
	currentRow = 0,
	blankLetters: string[] = [],
	yellowLetters: Record<string, number[]> = {},
	selectedWords: string[] = []
) {
	for (let i = 0; i < wordNodes[currentRow].length; i++) {
		let nextWord: string = wordNodes[currentRow][i];

		if (naturallyFollows(currentWord, nextWord, blankLetters, yellowLetters, standards)) {
			let newBlankLetters: string[] = [...blankLetters]; // Copy the blankLetters array
			let newYellowLetters: Record<string, number[]> = { ...yellowLetters }; // Copy the yellowLetters array
			let newSelectedWords: string[] = [...selectedWords, nextWord]; // Add the nextWord to the selectedWords array

			for (let i = 0; i < 5; i++) {
				if (wordleArray[currentRow][i] === "B") {
					newBlankLetters.push(nextWord[i]);
				}
				if (wordleArray[currentRow][i] === "Y") {
					if (newYellowLetters[nextWord[i]]) {
						newYellowLetters[nextWord[i]].push(i);
					} else {
						newYellowLetters[nextWord[i]] = [i];
					}
				}
			}

			if (currentRow === wordNodes.length - 2) {
				return newSelectedWords;
			} else {
				let result = depthWordleSearch(
					nextWord,
					wordNodes,
					wordleArray,
					standards,
					currentRow + 1,
					newBlankLetters,
					newYellowLetters,
					newSelectedWords
				);
				if (result) {
					return result;
				}
			}
		}
	}
	// console.log("No solution found at" + currentWord + " " + currentRow + selectedWords + blankLetters + yellowLetters)
	return false;
}

function naturallyFollows(currentWord, targetWord, blankLetters, yellowLetters, standards) {
	if (currentWord === targetWord && !standards.allowRepeats) {
		return false;
	}

	if (currentWord === "") {
		return true;
	}

	for (let i = 0; i < 5; i++) {
		if (blankLetters.includes(targetWord[i]) && !standards.blankRepeats) {
			return false;
		}
	}

	for (let i = 0; i < Object.keys(yellowLetters).length; i++) {
		const yellowLetter = Object.keys(yellowLetters)[i];

		// if we've seen a yellow square before, we expect to see it again
		if (!standards.yellowNeglect) {
			if (!targetWord.includes(yellowLetter)) {
				return false; //First to remove for sure
			}
		}
		// We expect users to place yellow letters in new squares
		if (!standards.repeats) {
			for (let j = 0; j < yellowLetters[yellowLetter].length; j++) {
				const visitedPosition = yellowLetters[yellowLetter][j];
				if (targetWord[visitedPosition] === yellowLetter) {
					return false;
				}
			}
		}
	}
	return true;
}




globalJsonData = await fetchData();

const textField = document.getElementById("wordle-input") as HTMLTextAreaElement;
textField.addEventListener("focus", function () {
  if (true) {
    textField.textContent = "";
  }
});

function extraOptions() {
  const extraFields = document.getElementById("extraFields") as HTMLDivElement;
  extraFields.style.display =
    extraFields.style.display === "block" ? "none" : "block";
}

const form = document.getElementById("wordle-form");
if (form) {
  form.addEventListener("submit", function (event) {
    event.preventDefault();
    const input = document.getElementById("wordle-input") as HTMLTextAreaElement;
    const startingWords = document.getElementById("startingWords")
      ? (document.getElementById("startingWords") as HTMLTextAreaElement).value
      : "";
    const customAnswer = document.getElementById("customAnswer")
      ? (document.getElementById("customAnswer") as HTMLInputElement).value
      : "";
    const results = document.getElementById("output") as HTMLDivElement;
    while (results.firstChild) {
      results.removeChild(results.firstChild);
    }
    if (globalJsonData && globalJsonData.words) {
      const output = parseWordle(input, startingWords, customAnswer);

      if (output) {
        const outputDOM = parsedWordleToDOM(output);
        results.appendChild(outputDOM);
      }
    } else {
      alert("Couldn't find the list of legal words.");
    }
  });
}

function parsedWordleToDOM(wordleObject) {
  const container = document.createElement("div");
  container.className = "container";

  const header = document.createElement("div");
  header.className = "header";

  const title = document.createElement("div");
  title.className = "resultsTitle";
  title.textContent = `Wordle #${wordleObject.wordleNumber} ${wordleObject.score}`;

  const date = document.createElement("div");
  date.className = "resultsDate";
  date.textContent = `${wordleObject.date}`;

  const answer = document.createElement("div");
  answer.textContent = `Answer: ${wordleObject.answer}`;

  const matches = document.createElement("div");
  matches.textContent = `Matching words: ${wordleObject.matches.toLocaleString()}`;

  const permutations = document.createElement("div");
  permutations.textContent = `Permutations: ${wordleObject.permutations.toLocaleString()}`;

  const extraInfo = document.createElement("pre");
  extraInfo.innerText = JSON.stringify(wordleObject, null, 2);
  extraInfo.style.display = "none";

  const button = document.createElement("button");
  button.textContent = "+";
  button.className = "extraInfo";
  button.addEventListener("click", () => {
    // Toggle the visibility of the JSON elements
    extraInfo.style.display =
      extraInfo.style.display === "none" ? "block" : "none";
    button.textContent = button.textContent === "+" ? "–" : "+";
  });

  header.appendChild(title);
  header.appendChild(date);
  header.appendChild(answer);

  header.appendChild(matches);
  header.appendChild(permutations);
  header.appendChild(button); // Add the button here
  header.appendChild(extraInfo); // Add the extra info here

  container.appendChild(header);

  return container;
}

function parseWordle(input: HTMLTextAreaElement, startingWords, customAnswer) {
  console.log(input)
  const lines = input.value?.trim().split("\n");
  console.log(lines)
  const header = lines[0].split(" ");
  console.log(header)
  const wordleNumber = parseInt(header[1].replace(/,/g, ""));
  console.log(wordleNumber)
  const score = header[2].replace("*", "");
  console.log(score)
  const hardMode = header[2].includes("*");

  const grid = lines.slice(1);
  console.log(grid)
  const linear = grid.join("").replace(/[\n\s]/g, "");
  console.log(linear)
  const firstDate = new Date(2021, 5, 19);
  const rawDate = new Date(
    firstDate.setDate(firstDate.getDate() + wordleNumber),
  );
  const today = new Date();
  if (rawDate > today) {
    alert("That's a future wordle!");
    return false;
  }
  const date = rawDate.toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  let plainText = "";

  for (let i = 0; i < linear.length; i++) {
    const char = linear[i].charCodeAt(0);
    if (char === 11035) {
      plainText += "B";
    }
    if (char === 57320) {
      plainText += "Y";
    }
    if (char === 57321) {
      plainText += "G";
    }
  }

  let answer = "";
  if (customAnswer) {
    answer = customAnswer.toUpperCase();
  } else if (globalJsonData && globalJsonData.answers) {
    if (globalJsonData.answers[wordleNumber]) {
      answer = globalJsonData.answers[wordleNumber];
    } else {
      alert(
        "We weren't able to get this wordle. The database may be late to update.",
      );
      return false;
    }
  } else {
    alert(
      "We weren't able to get the updated answers list. Try again or enter a custom answer.",
    );
    return false;
  }

  if (startingWords) {
    //split words by spaces, newlines, or commas. All words must be 5 letters long
    const words = startingWords
      .split(/[\s,]+/)
      .filter((word) => word.length === 5) as string[];
    for (let i = words.length - 1; i >= 0; i--) {
      console.log(words[i].toUpperCase());
      globalJsonData.words.unshift(words[i].toUpperCase());
    }
  }

  const [solutions, indexes, permutations, checks, matches, standardsMet] =
    reverseSolver(plainText, answer, globalJsonData.words);

  if (standardsMet === -1) {
    const noAnswer = document.createElement("div");
    noAnswer.textContent = "No answer found!";
    noAnswer.style.margin = "0 auto";
    const resultsScreen = document.getElementById("resultsScreen");
    if (!resultsScreen) {
      return false;
    }
    resultsScreen.style.justifyContent = "center";
    const output = document.getElementById("output");
    if (!output) {
      return false;
    }
    output.appendChild(noAnswer);
    output.style.textAlign = "center";
    return false;
  } else {
    const resultsScreen = document.getElementById("resultsScreen");
    if (!resultsScreen) {
      return false;
    }
    resultsScreen.style.justifyContent =
      "space-around";
  }

  createGrid(solutions, indexes, plainText);

  return {
    wordleNumber,
    score,
    hardMode,
    plainText,
    answer,
    date,
    permutations,
    checks,
    matches,
    standardsMet,
  };
}

function generateRandomColor(color) {
  if (color === "G") {
    return "#538D4E";
  }
  if (color === "Y") {
    return "#B59F3B";
  }
  return "#3A3A3C";
}

function createRow(solutions, startingIndex, colors, rowNumber) {
  const row = document.createElement("div");

  row.classList.add("row");

  const leftArrow = document.createElement("div");
  leftArrow.className = "arrow";
  leftArrow.textContent = "<";
  row.appendChild(leftArrow);

  for (let i = 0; i < 5; i++) {
    const square = document.createElement("div");
    square.className = "square";
    square.textContent = solutions[startingIndex][i];
    square.style.backgroundColor = generateRandomColor(colors[i]); // Set background color
    row.appendChild(square);
  }

  const rightArrow = document.createElement("div");
  rightArrow.className = "arrow";
  rightArrow.textContent = ">";
  row.appendChild(rightArrow);

  const indexLabel = document.createElement("div");
  indexLabel.className = "indexLabel";
  indexLabel.textContent = `${startingIndex + 1}/${solutions.length}`;
  row.appendChild(indexLabel);

  leftArrow.addEventListener("click", () =>
    changeRowContents(row, rowNumber, -1, solutions),
  );
  rightArrow.addEventListener("click", () =>
    changeRowContents(row, rowNumber, 1, solutions),
  );

  return row;
}

function changeRowContents(row, rowNumber, direction, solutions) {
  const squares = row.querySelectorAll(".square") as NodeListOf<HTMLDivElement>;
  const indexLabel = row.querySelector(".indexLabel") as HTMLDivElement;
  const letters = Array.from(squares).map((square) => square.textContent);
  const word = letters.join("");
  const currentIndex = solutions.indexOf(word);
  let newIndex = (currentIndex + direction) % solutions.length;
  if (newIndex < 0) {
    newIndex = solutions.length - 1;
  }
  if (newIndex >= solutions.length) {
    newIndex = 0;
  }
  indexLabel.textContent = `${newIndex + 1}/${solutions.length}`;
  const newWord = solutions[newIndex];

  const colors = Array.from(squares).map(
    (square) => square.style.backgroundColor,
  ); // Store background colors

  // Flip out, once animation is finished remove flip-out class, switch letters, then flip in
  for (let i = 0; i < squares.length; i++) {
    squares[i].classList.add("flip-out-animation");
    squares[i].addEventListener("animationend", () => {
      squares[i].classList.remove("flip-out-animation");
      squares[i].textContent = newWord[i];
      squares[i].classList.add("flip-in-animation");
      squares[i].addEventListener("animationend", () => {
        squares[i].classList.remove("flip-in-animation");
      });
    });
  }
}

function createGrid(solutions, indexes, colorText) {
  const numRows = Math.floor(colorText.length / 5);
  const colorsArray = colorText.match(/.{1,5}/g);
  const gridContainer = document.getElementById("grid-container");
  if (!gridContainer) {
    return;
  }
  gridContainer.style.maxHeight = `${numRows * 66}px`;
  // Clear the existing grid
  while (gridContainer.firstChild) {
    gridContainer.removeChild(gridContainer.firstChild);
  }
  for (let i = 0; i < numRows; i++) {
    gridContainer.appendChild(
      createRow(solutions[i], indexes[i], colorsArray[i], i),
    );
  }
}
