package main

import (
	"io"
	"strconv"
	"text/tabwriter"

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
