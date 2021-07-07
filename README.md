# Strategy service

Strategy service uses same singleton pattern around managing runtime of database model (Strategy)

Implemented strategies:
 - Smart-order

---
### Testing

``
go test -v ./tests
``

---
### Run

``
go build main
``

---
# Try Out Development Containers: Go

This is a sample project that lets you try out the **[VS Code Remote - Containers](https://aka.ms/vscode-remote/containers)** extension in a few easy steps.

> **Note:** If you're following the quick start, you can jump to the [Things to try](#things-to-try) section. 

## Setting up the development container

Follow these steps to open this sample in a container:

1. If this is your first time using a development container, please follow the [getting started steps](https://aka.ms/vscode-remote/containers/getting-started).

2. If you're not yet in a development container:
   - Clone this repository.
   - Press <kbd>F1</kbd> and select the **Remote-Containers: Open Folder in Container...** command.
   - Select the cloned copy of this folder, wait for the container to start, and try things out!

## Things to try

Once you have this sample opened in a container, you'll be able to work with it like you would locally.

Some things to try:

1. **Edit:**
   - Open `server.go`
   - Try adding some code and check out the language features.
2. **Terminal:** Press <kbd>ctrl</kbd>+<kbd>shift</kbd>+<kbd>\`</kbd> and type `uname` and other Linux commands from the terminal window.
2. **Build, Run, and Debug:**
   - Open `server.go`
   - Add a breakpoint (e.g. on line 22).
   - Press <kbd>F5</kbd> to launch the app in the container.
   - Once the breakpoint is hit, try hovering over variables, examining locals, and more.
   - Continue, then open a local browser and go to `http://localhost:9000` and note you can connect to the server in the container.
3. **Forward another port:**
   - Stop debugging and remove the breakpoint.
   - Open `server.go`
   - Change the server port to 5000. (`portNumber := "5000"`)
   - Press <kbd>F5</kbd> to launch the app in the container.
   - Press <kbd>F1</kbd> and run the **Remote-Containers: Forward Port from Container...** command.
   - Select port 5000.
   - Click "Open Browser" in the notification that appears to access the web app on this new port.
  
## Scaling

The `strategy_service` supports multiple instances running at the same time.
Each strategy will have not more than 1 `strategy_service` instance running strategy's runtime.

To consider details, at first let's define a couple of definitions.

* Strategy runtime - dynamic process defined by strategy parameters values that `strategy_service` provides to let
  a strategy change it's state and place orders. If `strategy_service` instance received strategy from database, started
  it and sends orders it requires, means it settled strategy and holds strategy's runtime.
* Strategy settling - a process when `strategy_service` instance checks if any other instances have a runtime running
  for the strategy and starts it in case no other instances have it.
* Homeless strategy - a strategy with `enabled` status we have in MongoDB, but no instance of `strategy_service` holds a
  runtime for it.

Now there are three things to consider:

1. There is `MODE` environment variable defines what strategies current instance should take to settle a runtime (check
   details below),
2. Only one instance will settle the strategy,
3. Instance stops strategies settling once CPU load average or RAM limit reached and settles new strategies again after
   resources become available. NB! Current implementation does not check for homeless strategies stored in database
   after resources became available. It only takes new strategies written to MongoDB.
   
### Running multiple instances at the same time

No problem with multiple instances running at the same time.
Just ensure all possible strategies covered by `MODE`s specified for current set of application instances running.

### Up-scaling

One can track CPU load average and RAM usage and run another instance of `strategy_service` when current instance is
close to resources limit.
Check StrategyService.runIsFullTracking function to find current limits used.

### Downscaling

While current implementation is not aware of homeless strategies already written to MongoDB, it is important to init new
instance of `strategy_service` to settle strategies from semi-empty instances terminated. Otherwise strategies from
terminated instances will stay homeless.

## Modes

Currently, there are three modes supported.
Mode specified by `MODE` environment variable.

`MODE` value | Behavior
-------------|---------
Not set, set to empty string "" or "All" | Instance considers all strategies for settling.
"Bitcoin" | Strategies with `BTC` substring in pair name considered for settling. Other ignored.
"Altcoins" | All strategies, but those have no `BTC` substring in pair name, considered for settling.
"ADA_USDT" | Strategies with "ADA_USDT" pair considered for settling. Other ignored (handy for debugging).

Any other values lead to application crash on initialization.
