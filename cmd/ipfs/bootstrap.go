package main

import (

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	//"github.com/jbenet/go-ipfs/core/commands"
	"fmt"
    "io/ioutil"
   "encoding/json"
	u "github.com/jbenet/go-ipfs/util"
    "strings"
	"os"
//	"io"
"bufio"
	
)


var cmdIpfsBootstrap = &commander.Command{
	UsageLine: "bootstrap",
	Short:     "Show a list of bootsrapped addresses.",
	Long: `ipfs bootstrap <add/remove> <address>... - show/add/remove bootstrapped addresses

	Shows a list of bootstrapped addresses. use 'add' or 'remove' followed
	by a specified <address> to add/remove it from the list.
`,
	Run:  bootstrapCmd,
	Flag: *flag.NewFlagSet("ipfs-bootstrap", flag.ExitOnError),
}


type Peer struct {
	Address string
	PeerID  string
	
}

type Config struct {
	Bootstrap []Peer
}



func bootstrapCmd(c *commander.Command, inp []string) error {
	
	if len(inp) == 0 {
		
		configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
		    dat, _ := ioutil.ReadFile(configpath)
		    var configText = string(dat)


		 var conf Config
		  	 err := json.Unmarshal([]byte(configText), &conf)

		 	 if err != nil {
		 		fmt.Print("Error:", err)
		 	 }


		 	 //printing list of peers
		 	for i, _ := range conf.Bootstrap {
		 	    s := []string{conf.Bootstrap[i].Address, "/", conf.Bootstrap[i].PeerID, "\n"}
		 	     fmt.Printf(strings.Join(s, ""))
		 	}

		return nil
		
	    }
		
		
	  switch arg := inp[0]; arg {
	      case "add":
			  if len(inp) == 1 {
				  fmt.Println("No peer specified.")
				  return nil
			  }
			  
			  //if text doesn't contain /
			  
			  
			  	
			  

			  	var stringArr = strings.SplitAfterN(inp[1], "/", 6)
		 	    s := []string{stringArr[0], stringArr[1], stringArr[2], stringArr[3], stringArr[4]}
				var peerID = stringArr[5]
                var addressPretrim = strings.Join(s, "")
				var address = strings.TrimSuffix(addressPretrim, "/")
				//bootstrap object created of user entered peer data
  				peer := Peer{
	  				  		PeerID:    peerID,
	  					  	Address:   address,


	  				}
					b, err := json.Marshal(peer)
					if (err != nil) {
						panic(err)
					}
					
					configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
					
					err2 := AddPeertoFile(configpath, b)
					if(err2 != nil) {
						panic(err)
					}
				
				
				
			  return nil
	      case "remove":
			  if len(inp) == 1 {
				  fmt.Println("No peer specified.")
				  return nil
			  }
			  
			 
			  if strings.Contains(inp[1], "/") {
			 
		  	  var stringArr = strings.SplitAfterN(inp[1], "/", 6)
	 	      s := []string{stringArr[0], stringArr[1], stringArr[2], stringArr[3], stringArr[4]}
			  var peerID = stringArr[5]
              var address = strings.Join(s, "")
			  
			  
		    	configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
		    	err2 := RemoveSpecificPeerfromFile(configpath, peerID, address)
		    	if(err2 != nil) {
		    		panic(err2)
			    }
			  	
			}
			
			if !strings.Contains(inp[1], "/") {
				
  		    	configpath, _ := u.TildeExpansion("~/.go-ipfs/config")
  		    	err2 := RemovePeerfromFile(configpath, inp[1])
  		    	if(err2 != nil) {
  		    		panic(err2)
  			    }
			}		  
			  
			  return nil
	  }
	
	  return nil

 }
 
 func AddPeertoFile(filename string, peerData []byte) error {
     // open the file
     file, err := os.Open(filename)
     if err != nil {
         return err
     }
     // get the file permissions (for later)
     info, err := file.Stat()
     if err != nil {
         return err
     }
     perm := info.Mode()
     // read the file line by line
     lines := []string{}
     for scanner := bufio.NewScanner(file); scanner.Scan(); {
         lines = append(lines, scanner.Text())
     }
     // close the file
     if err = file.Close(); err != nil {
         return err
     }
	 
	 //write it only once
	 var x = 0
	 //find line with ] 
	 for i, line := range lines {
		 if x == 0 {
		 if(strings.ContainsRune(line, ']')) {
			 //take the line before... and append/write to it
			 
			 // make the slice longer
			 lines = append(lines, "")
			 // shift each element back
			 copy(lines[i+1:], lines[i:])
			 // now you can insert the new line at i
			 
			 s := string(peerData)
			 c := byte(',')
			 var appendedPeer = string(c)
 			 appendedPeer += s
			 
			 lines[i] = string(appendedPeer)
			 fmt.Println("Added Peer to Config")
			 x++
		 }
	 }
	 
		 
	 }
	
	
     
     // O_TRUNC will truncate the file upon open
     file, err = os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, perm)
     if err != nil {
         return err
     }
     defer file.Close()
     // write the lines back to the file
     for _, line := range lines {
         if _, err = file.WriteString(line + "\n"); err != nil {
             return err
         }
     }
     return nil
 }
 
 func RemoveSpecificPeerfromFile(filename string, peerID string, address string) error {
     // open the file
     file, err := os.Open(filename)
     if err != nil {
         return err
     }
     // get the file permissions (for later)
     info, err := file.Stat()
     if err != nil {
         return err
     }
     perm := info.Mode()
     // read the file line by line
     lines := []string{}
     for scanner := bufio.NewScanner(file); scanner.Scan(); {
         lines = append(lines, scanner.Text())
     }
     // close the file
     if err = file.Close(); err != nil {
         return err
     }
   
	
	
	 //find line with peer data 
	 for i, line := range lines {
		 if(strings.Contains(line, peerID) && strings.Contains(line,address)) {
			 
		
			 fmt.Println("FOUND IT!" , i )
			  //remove it 
			     lines = append(lines[:i], lines[i+1:]...)
		 }
	 }
 

     
     // O_TRUNC will truncate the file upon open
     file, err = os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, perm)
     if err != nil {
         return err
     }
     defer file.Close()
     // write the lines back to the file
     for _, line := range lines {
         if _, err = file.WriteString(line + "\n"); err != nil {
             return err
         }
     }
     return nil
 }
 
 func RemovePeerfromFile(filename string, address string) error {
     // open the file
     file, err := os.Open(filename)
     if err != nil {
         return err
     }
     // get the file permissions (for later)
     info, err := file.Stat()
     if err != nil {
         return err
     }
     perm := info.Mode()
     // read the file line by line
     lines := []string{}
     for scanner := bufio.NewScanner(file); scanner.Scan(); {
         lines = append(lines, scanner.Text())
     }
     // close the file
     if err = file.Close(); err != nil {
         return err
     }
   
	
	
	 //find line with peer data 
	 for i, line := range lines {
		 if(strings.Contains(line, address))  {
			 fmt.Println("FOUND IT!" , i )
			  //remove it 
			     lines = append(lines[:i], lines[i+1:]...)
		 }
	 }
 

     
     // O_TRUNC will truncate the file upon open
     file, err = os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, perm)
     if err != nil {
         return err
     }
     defer file.Close()
     // write the lines back to the file
     for _, line := range lines {
         if _, err = file.WriteString(line + "\n"); err != nil {
             return err
         }
     }
     return nil
 }
 

