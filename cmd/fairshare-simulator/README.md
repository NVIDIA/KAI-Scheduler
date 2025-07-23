# Fairshare Simulator

This is a simple HTTP server that simulates the fair share resource division algorithm used in the KAI Scheduler's proportion plugin.

## Building and Running

Build the simulator:

```bash
go build .
```

Run it:

```bash
./fairshare-simulator -port=8080
```

The port is configurable with the `-port` flag and defaults to 8080.

## Usage

Send a POST request to `/simulate` with a JSON body containing the simulation parameters.

### Example Request

```http
POST /simulate HTTP/1.1
Content-Type: application/json

{
    "totalResource": {
      "GPU": 100,
      "CPU": 16000,
      "Memory": 32000000
    },
    "queues": [
      {
        "uid": "queue1",
        "name": "test-queue",
        "priority": 0,
        "resourceShare": {
          "gpu": {
            "deserved": 10,
            "request": 100,
            "overQuotaWeight": 3
          }
        }
      },
      {
        "uid": "queue2",
        "name": "test-queue2",
        "priority": 0,
        "resourceShare": {
          "gpu": {
            "deserved": 10,
            "request": 100,
            "overQuotaWeight": 1
          }
        }
      }
    ]
}
```

### Response

The response is a JSON object with fair share values for each queue:

```json
{
  "queue1": {
    "gpu": 70,
    "cpu": 16000,
    "memory": 100000
  },
  "queue2": {
    "gpu": 30,
    "cpu": 16000,
    "memory": 100000
  }
}
```

(Note: Actual values depend on the input parameters and the simulation logic.)

This simulator uses the `SetResourcesShare` function from the proportion plugin to compute the fair shares. 

## Using the Jupyter Notebook Example

An interactive example is provided in `example.ipynb`. To run it:

1. Make sure you have Python with the required packages:
```bash
pip install requests pandas matplotlib jupyter
```

2. Start the simulator:
```bash
./fairshare-simulator -port=8080
```

3. In a new terminal, start Jupyter:
```bash
jupyter notebook example.ipynb
```

4. Run the notebook cells to:
   - Send example requests to the simulator
   - View the fair share results in a DataFrame
   - Visualize the resource allocation with bar charts

You can modify the `request_data` dictionary in the notebook to experiment with different configurations and see how they affect resource allocation. 