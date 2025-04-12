# Basic Ollama proxy

- Reverse proxy for 2 hosts, always prioritises host A

# Why ?

- I have 1 GPU and want to run LLMs on my server, but also game on my desktop
- I also want to run smaller AI models 24/7

# Huh ?

This program allows me to use Server A ( My desktop with a large GPU in it ) _whenever_ it's online (for ollama),
and fallback to Server B ( My actual server) whenever it's offline ( alot )
