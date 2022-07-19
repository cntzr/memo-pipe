# memo-pipe
A pipeable tool chain for persistent memos. There are a few ideas behind this setup. First there are 
my continous trials over the years to build a virtual persona like Napoleon from Timothy Truckle. 
I hope they have become more realistic over the years. Second there is a long-time love for linux 
and the terminal. When I'm sitting on a linux computer most of the time I'm looking on terminal 
windows. I even do all my programming in the terminal using an improved instance of neovim. And last but not least 
there are a few realy good books from John Arnundel all about Go programming. He put me back to the 
basics ... simple tools, no solutions without problems, test driven development and an awesome 
library for scripting with Go.

At the moment it's only about memos. I discarded my first attempt to make it generic. I can ...

* ✓ pipe a text via stdout,
* ✓ tag it, maybe multiple times,
* ✓ store it into a file and
* ✘ maintain an index for all memos.

On the way back somebody can ...

* ✓ ask the index for all memos in a period,
* ✓ filter them for some tags and
* ✓ print them onto the terminal.

The periods have to be described verbally. Dates or timestamps are not supported. Following periods are 
supported by now ...

* ✓ all (default),
* ✘ today,
* ✘ yesterday,
* ✘ thisweek,
* ✘ lastweek,
* ✘ last2weeks,
* ✘ thismonth,
* ✘ lastmonth,
* ✘ last2months,
* ✘ thisyear,
* ✘ lastyear and
* ✘ last2years.

