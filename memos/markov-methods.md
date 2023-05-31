The general methods of solving time-to or absorption-likelihood are easy to remember. That is, to solve for the expected time until first entering a state $s$, we compute variables $x_i$ signifiying the expected time until entering state $s$, when starting from state $i$. Then, necessarily, this reduces to a linear algebra problem with $n$ as the total number of states.

Similarily, to compute the expected time until absorption at some component or node, we may compute variables $x_i$ for the same metric, when starting at state $i$, and solve a linear system.

> Question: 
