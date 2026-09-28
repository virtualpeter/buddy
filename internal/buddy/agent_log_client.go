package buddy

import "buddy/internal/buddy/agent"

func init() {
	// Token requests name the caller in the Agent activity log. The source is
	// live, so an IDE session that switches the channel is named on the next mint.
	agent.SetChannelSource(ActiveChannel)
}
