import { useEffect, useState } from 'react';

const GREETINGS = [
  'hi there',
  'hello hello',
  'ready when you are',
  "let's build something",
  'boop',
  'morning, brain',
  'awake and caffeinated',
  'tinkering...',
  'thinking thoughts',
  'humming gently',
  'pondering pixels',
  'compiling vibes',
  '>.<',
  '^_^',
  '(•‿•)',
  '(づ｡◕‿‿◕｡)づ',
  '¯\\_(ツ)_/¯',
  '◕ ◡ ◕',
  '(｡◕‿‿◕｡)',
  '(◍•ᴗ•◍)',
  '~( ˘▾˘~)',
  '(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧',
  'σ(･ε･`)',
  '( ˘ ³˘)♥',
];

const ROTATE_MS = 5200;

function pickStart(): number {
  return Math.floor(Math.random() * GREETINGS.length);
}

export function PlayfulGreeting() {
  const [index, setIndex] = useState(pickStart);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const id = window.setInterval(() => {
      setIndex((i) => (i + 1) % GREETINGS.length);
      setTick((t) => t + 1);
    }, ROTATE_MS);
    return () => window.clearInterval(id);
  }, []);

  function reroll() {
    setIndex((i) => (i + 1) % GREETINGS.length);
    setTick((t) => t + 1);
  }

  const phrase = GREETINGS[index];

  return (
    <h1
      className="greeting"
      onClick={reroll}
      title="click for another"
      aria-live="polite"
    >
      <span key={tick} className="greeting-phrase">
        {phrase}
      </span>
    </h1>
  );
}
