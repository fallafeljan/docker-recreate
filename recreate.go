package main

import (
  "fmt"
  "os"
  "strconv"
  "strings"
  "time"

  "github.com/fsouza/go-dockerclient"
  //"github.com/tonnerre/golang-pretty"
)

func checkError(err error) {
  if err != nil {
    fmt.Println(err)
    os.Exit(0)
  }
}

func parseImageName(imageName string) (repository string, tag string) {
  sepIndex := strings.LastIndex(imageName, ":")

  if sepIndex > -1 {
    repository := imageName[:sepIndex]
    tag := imageName[(sepIndex+1):]

    return repository, tag
  } else {
    return imageName, "latest"
  }
}

func main() {
  if len(os.Args) < 2 {
    fmt.Printf("Usage: %s [-p] id [tag]\n", os.Args[0])
    os.Exit(0)
  }

  client, err := docker.NewClientFromEnv()
  checkError(err)

  pullImage := os.Args[1] == "-p"
  containerId := os.Args[len(os.Args) - 1]
  desiredTag := ""

  if len(os.Args) == 4 {
    containerId = os.Args[2]
    desiredTag = os.Args[3]
  }

  oldContainer, err := client.InspectContainer(containerId)
  checkError(err)

  // TODO delete _new if an error occures

  repository, currentTag := parseImageName(oldContainer.Config.Image)

  if desiredTag == "" {
    desiredTag = currentTag
  }

  fmt.Printf("Image: %s:%s\n", repository, desiredTag)

  if pullImage {
    fmt.Print("Pulling image...\n")

    err = client.PullImage(docker.PullImageOptions{
      Repository: repository,
      Tag: desiredTag }, docker.AuthConfiguration{})

    checkError(err)
  }

  // TODO handle image tags/labels?

  now := int(time.Now().Unix())
  then := now - 1

  name := oldContainer.Name
  temporaryName := name + "_" + strconv.Itoa(now)

  // TODO possibility to add/change environment variables
  var options docker.CreateContainerOptions
  options.Name = temporaryName
  options.Config = oldContainer.Config
  options.Config.Image = repository + ":" + desiredTag
  options.HostConfig = oldContainer.HostConfig
  options.HostConfig.VolumesFrom = []string{oldContainer.ID}

  links := oldContainer.HostConfig.Links

  for i := range links {
    parts := strings.SplitN(links[i], ":", 2)
    if len(parts) != 2 {
      fmt.Println("Unable to parse link ", links[i])
      // TODO make function and add better error return
      return
    }

    containerName := strings.TrimPrefix(parts[0], "/")
    aliasParts := strings.Split(parts[1], "/")
    alias := aliasParts[len(aliasParts)-1]
    links[i] = fmt.Sprintf("%s:%s", containerName, alias)
  }
  options.HostConfig.Links = links

  fmt.Println("Creating...")
  newContainer, err := client.CreateContainer(options)
  checkError(err)

  err = client.RenameContainer(docker.RenameContainerOptions{
    ID: oldContainer.ID,
    Name: name + "_" + strconv.Itoa(then) })
  checkError(err)

  err = client.RenameContainer(docker.RenameContainerOptions{
    ID: newContainer.ID,
    Name: name})
  checkError(err)

  if oldContainer.State.Running {
    fmt.Printf("Stopping old container\n")
    err = client.StopContainer(oldContainer.ID, 10)
    checkError(err)

    fmt.Printf("Starting new container\n")
    err = client.StartContainer(newContainer.ID, newContainer.HostConfig)
    checkError(err)
  }

  // TODO fallback to old container if error occured
  // TODO add option to remove old container on sucsess

  fmt.Printf(
    "Migrated from %s to %s\n",
    oldContainer.ID[:4],
    newContainer.ID[:4])

  fmt.Println("Done")
}
