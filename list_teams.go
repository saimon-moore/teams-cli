package main

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	teams_api "github.com/saimon-moore/teams-api"
	"github.com/saimon-moore/teams-api/pkg/csa"
)

func formatTeamsTable(out io.Writer, teams []csa.Team) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := writer.Write([]byte("NAME\tID\tFAVORITE\tFOLLOWED\tARCHIVED\tDELETED\tCHANNELS\n")); err != nil {
		return err
	}

	for _, team := range teams {
		row := team.DisplayName + "\t" +
			team.Id + "\t" +
			strconv.FormatBool(team.IsFavorite) + "\t" +
			strconv.FormatBool(team.IsFollowed) + "\t" +
			strconv.FormatBool(team.IsArchived) + "\t" +
			strconv.FormatBool(team.IsDeleted) + "\t" +
			strconv.Itoa(len(team.Channels)) + "\n"
		if _, err := writer.Write([]byte(row)); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func runListTeams(out io.Writer) error {
	client, err := teams_api.New()
	if err != nil {
		return fmt.Errorf("unable to initialize teams client: %v", err)
	}

	state := TeamsState{teamsClient: client}
	data, err := state.fetchConversationData()
	if err != nil {
		return err
	}

	return formatTeamsTable(out, data.conversations.Teams)
}
