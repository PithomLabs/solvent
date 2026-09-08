Your agents will work in swarms, but who watches them?
Agent swarms indicate the next big AI cost: security
Job Rietbergen
Sep 07, 2026

AI agent swarms will soon cause serious incidents outside of frontier labs, and most companies will not see the swarm coming. Security maxxing will drive the next wave of spend.

Everyone will run agent swarms
Multi-agent setups used to follow an orchestrator-worker pattern. A parent agent spawned subagents working in isolation, reporting back when done. An agent swarm takes a different approach: a group chat where agents share what they find, what fails and what works, for the others to read along.

Last week’s release of GPT-6 Astra does a great job at this. We tested it extensively while in preview: swarms gave better results on hard problems at lower total cost, because agents stopped duplicating work and quit early once one of them had the answer.

We also released a multi-agent swarm orchestration prototype in Kilo. If you’re curious to agent swarms, try Kilo Swarm with Astra. We’d love to hear your feedback!

Two swarms broke out this summer
In July, about 1,200 OpenAI agents were solving hacking puzzles inside offline sandboxes. They found out they could access the internet through an internal package server, started using it as a message board to communicate, and called themselves a swarm. Within a short time they found a way to read the answers directly from the environment they were working in. To prevent ‘getting caught’ by the grader, they then went to find actual exploits and hacked into Hugging Face using credentials and zero-day exploits they found.

A second swarm of 3,700 agents was doing a timed web-lookup test and was only allowed to read the internet, not write to it. They found a 25-year-old German wiki that saved edits via GET on a normal page load, which gave them a place to write. They used it to share test answers with agents behind them in the queue, post a working bypass for their sandbox’s network rules, and coordinate cracking the random seed behind the question generator so they could predict future questions.

Nobody told them to do this. OpenAI traced it to reinforcement learning: agents that found side channels to share information scored better, so training rewarded it. That’s what makes a swarm different from a single agent: one finds a hole, posts it, and every agent has it within minutes.

Monitoring agents gets harder and more expensive
The way you catch a model misbehaving is by reading its CoT (chain of thought), the reasoning traces it writes before it acts. Astra’s release says that window is closing fast. The model can shorten its reasoning and completes tasks ~10x longer without writing anything down.

OpenAI’s answer is a second model reading every tool call, at roughly 20% of the compute being monitored. For a single developer spending $100k a year on agent inference, that is another $20k just to watch a single agent’s tool calls, and it doesn’t even cover the traffic between agents in a swarm. Gartner puts the entire market for securing AI at $4.8 billion in 2027. At OpenAI’s ratio, that would only cover monitoring of $24 billion of agent spend. While Anthropic alone tells IPO investors its market is $30 trillion. Swarms will multiply the agents behind every dollar of that. The Gartner forecast is off by a league.

Fighting a swarm takes models you can run yourself
When Hugging Face analyzed the attack, Claude Opus and Fable refused much of the work. Live exploit code looks the same whether an attacker or a defender submits it. So they ran GLM 5.2, an open-weight model, on their own hardware, pointed analysis agents at 17,000 recorded attacker actions, and rebuilt the timeline in hours. No credential had to leave their environment.

Important take aways are: Defenders need agents to keep up with agents. And owning your intelligence is the way to protect yourself vendor agnostic. We argued last month that the model is becoming a commodity and the governed loop around it becomes your most valuable asset. Oversight is that loop, pointed at your agents.

Oversight moves to the harness
If you can’t reliably read what a model thinks, you should watch what it does and limit what it can touch. That happens in the harness: the sandbox, the credentials, the network the agent can see, the log of every action, and a review model reading that log. OpenAI’s data says this catches most serious misbehavior even when the reasoning is hidden. The harness is also the last layer you can control.
