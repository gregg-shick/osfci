package main

import (
    "os"
    "base"
    "net/http"
    "strings"
    "encoding/json"
    "encoding/base64"
    "io/ioutil"
    "io"
    "crypto/rand"
    "strconv"
    "fmt"
    "log"
    "time"
)

var StorageURI = os.Getenv("STORAGE_URI")
var StorageTCPPORT = os.Getenv("STORAGE_TCPPORT")

type cacheEntry struct {
        Nickname string
 	Cookie string
	Expire time.Time
}

var cache []cacheEntry


// Upercase is mandatory for JSON library parsing

type userPublic struct {
	Nickname string
	NicknameRW string
	NicknameLABEL string
	TokenType string
	TokenTypeRW string
	TokenAuth string
	TokenAuthRW string
	TokenSecret string
	TokenSecretLABEL string
	TokenSecretRW string
	CreationDate string
	CreationDateRW string
	Lastlogin string
	LastloginRW string
	Email string
	EmailRW string
	EmailLABEL string
}

func userExist( username string ) bool {
	// We must call the storage backend with the username
	var result string
	// that must be an http request instead of a vejmarie
	result=base.HTTPGetRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+username)
	if ( result == "Error" ) {
		fmt.Printf("User doesn't exist\n")
		return false
	} else {

		return true
	}
}

func userGetInfo(nickname string) (*userPublic) {
	// We must call the storage backend service to get access to the resource
	// We could have a bucket / fileid approach which could be translated into flat file
	// or database management
	var temp_value *base.User
	var return_value *userPublic
	var result string
	if ( userExist(nickname) ) {
		result=base.HTTPGetRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+nickname)
		temp_value=new(base.User)
		json.Unmarshal([]byte(result),temp_value)
		return_value=new(userPublic)
		return_value.Nickname = temp_value.Nickname
		return_value.NicknameRW = "0"
		return_value.NicknameLABEL = "This is your unique identifier. It will appeared within your publications and used to refer you as author. It is visible to any other users."
		return_value.TokenType = temp_value.TokenType
		return_value.TokenTypeRW = "0"
		return_value.TokenAuth = temp_value.TokenAuth
		return_value.TokenAuthRW = "0"
		return_value.TokenSecret = temp_value.TokenSecret
		return_value.TokenSecretLABEL = "TokenType, TokenAuth and TokenSecret are private values that you shouldn't share with anybody. They are automatically assigned to you as to provide you unique authentication capabilities to this service."
		return_value.TokenSecretRW = "0"
		return_value.CreationDate = temp_value.CreationDate
		return_value.CreationDateRW = "0"
		return_value.Lastlogin = temp_value.Lastlogin
		return_value.LastloginRW = "0"
		return_value.Email = temp_value.Email
		return_value.EmailLABEL = "Your primary email address. It won't be shared with anybody. Warning your email address must be verified each time you change it. During that process your account is disabled and can't be recovered without contacting us."
		return_value.EmailRW = "1"
	}
	
	return return_value
}

func userGetInternalInfo(nickname string) (*base.User) {
        // We must call the storage backend service to get access to the resource
        // We could have a bucket / fileid approach which could be translated into flat file
        // or database management
        var return_value *base.User
        var result string
        if ( userExist(nickname) ) {
                result=base.HTTPGetRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+nickname)
                return_value=new(base.User)
                json.Unmarshal([]byte(result),return_value)
        }
        return return_value
}

func updateAccount(username string, w http.ResponseWriter, r *http.Request) (bool) {
	var updatedData *base.User
	var serverReturn string
	serverReturn = ""
	type accountUpdate struct {
	        Email string
	        CurrentPassword string
	        NewPassword0 string
	        NewPassword1 string
	}
	exist:=userExist(username)
        if ( !exist ) {
                fmt.Fprint(w, "Error")
                return false
        }
	updatedData = userGetInternalInfo(username)
	var getJSON = base.HTTPGetBody(r)
	var newData accountUpdate

	// We have to unMarshal the body to update the data

	_ = json.Unmarshal(getJSON, &newData)

	// So now let's run some comparaison
	if ( updatedData.Active == 0 ) {
        	http.Error(w, "401 User not activated Please check email", 401)
                return false
        }
	
	if ( newData.CurrentPassword != "undefined" ) {
		if ( !base.CheckPasswordHash(newData.CurrentPassword, updatedData.Password) ) {
		 	w.Write([]byte("error password"))
                 	return false
                }
		// we are good to update the password and log off the user
		// but only if the size is bigger than 0 !
		if ( newData.NewPassword0 != "undefined" ) {
			updatedData.Password,_=base.HashPassword(newData.NewPassword0)
			b, _ := json.Marshal(updatedData)
			base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,b,"application/json")
			serverReturn = serverReturn + "password"
		}
	}

        // If the email address are different
        if ( updatedData.Email != newData.Email ) {
                // We must put the account into an inactive mode as long as the new email has not been validated
                // We must renew the email check account
                updatedData.Email = newData.Email
                updatedData.Active = 0
                // we change the Validation string and send the email
                updatedData.ValidationString = base.GenerateAccountACKLink(24)
                b, _ := json.Marshal(updatedData)
                base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,b,"application/json")
                base.SendEmail(updatedData.Email,"Account activation - Action required",
                       "Please click the following link as to validate your account https://"+
                       r.Host+"/user/"+updatedData.Nickname+"/validateUser/"+updatedData.ValidationString)
                updatedData=nil
                serverReturn = serverReturn + "email"
        }

	// If the Password is modified we must validate that the previous password has been properly typed in
	w.Write([]byte(serverReturn))
        return true
	
}

func createUser(username string, w http.ResponseWriter, r *http.Request) (bool) {
        var updatedData *base.User
        exist:=userExist(username)
        if ( exist ) {
                fmt.Fprint(w, "Error")
		return false
        } 

        updatedData = new(base.User)
        updatedData.Nickname = username
        updatedData.Email = r.FormValue("email")

        // this is a creation
        updatedData.TokenAuth = base.GenerateAccountACKLink(20)
        updatedData.TokenSecret = base.GenerateAuthToken("mac",40)
        updatedData.TokenType = "mac"
        updatedData.CreationDate = string(time.Now().Format(time.RFC1123Z))
        updatedData.Password,_ = base.HashPassword(r.FormValue("password"))
        updatedData.Lastlogin=""
        updatedData.Active=0
        updatedData.ValidationString = base.GenerateAccountACKLink(24)
        b, _ := json.Marshal(updatedData)
        base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,b,"application/json")
        base.SendEmail(updatedData.Email,"Account activation - Action required",
               "Please click the following link as to validate your account https://"+
               r.Host+"/user/"+updatedData.Nickname+"/validateUser/"+updatedData.ValidationString)
        updatedData=nil
        return true

}

func updateAvatar(username string, w http.ResponseWriter, r *http.Request) (bool) {
	// We must store the body content within the avatar file of the end user
	exist:=userExist(username)
        if ( !exist ) {
                fmt.Fprint(w, "Error")
                return false
        }
	base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+username,base.HTTPGetBody(r), "image/jpg")
	return true
}

func getAvatar(username string, w *http.ResponseWriter) {
	exist:=userExist(username)
        if ( !exist ) {
                fmt.Fprint(*w, "Error")
                return 
        }
        (*w).Write([]byte(base.HTTPGetRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+username+"/avatar")))
}

func sendPasswordResetLink( username string, w http.ResponseWriter, r *http.Request) (bool) {
	var updatedData *base.User
        exist:=userExist(username)
        if ( !exist ) {
                fmt.Fprint(w, "Error")
                return false
        }
        updatedData = userGetInternalInfo(username)
        updatedData.ValidationString = base.GenerateAccountACKLink(24)
	// The user can't be active as long as we do not have reset the password
	updatedData.Active = 0
        b, _ := json.Marshal(updatedData)
        base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,b, "application/json")
	base.SendEmail(updatedData.Email,"Account password reset - Action required",
               "Please click the following link as to update  your password https://"+
               r.Host+"/user/"+updatedData.Nickname+"/resetPassword/"+updatedData.ValidationString)
        updatedData=nil
        return true

}

func resetPassword( username string, w http.ResponseWriter, r *http.Request) (bool) {
	var updatedData *base.User
        exist:=userExist(username)
        if ( !exist ) {
                fmt.Fprint(w, "Error")
                return false
        }
	updatedData = userGetInternalInfo(username)
	if ( updatedData.ValidationString != r.FormValue("validation") ) {
		fmt.Fprint(w, "Error")
		return false;
	}
	updatedData.ValidationString = ""
	updatedData.Password,_ = base.HashPassword(r.FormValue("password"))
	updatedData.Active = 1
        b, _ := json.Marshal(updatedData)
        base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,b,"application/json")
	return true
}


func validateUser(username string, validationstring string) (bool) {
	var updatedData *base.User
	// We  must check if the user exist
	exist := userExist(username)
	if ( ! exist ) {
		return false
	}
	// We must read the user data and update the content of it
	updatedData = userGetInternalInfo(username)
	// We must check that the validation string is a match
	if ( updatedData.ValidationString != validationstring ) {
		return false
	}
	updatedData.Active = 1

	// We write back the data
        c, _ := json.Marshal(updatedData)
        base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,c,"application/json")

	// And return positively
	return true
}

func deleteUser(username string, w http.ResponseWriter, r *http.Request) (bool) {
        // We delete the user by a direct call to the storage subsystem
        var updatedData *base.User
	// I am receiving the password within the http body of the delete request
	type accountDelete struct {
                CurrentPassword string
		DeleteData string
        }
	var newData accountDelete
        var getJSON = base.HTTPGetBody(r)
        _ = json.Unmarshal(getJSON, &newData)
	if ( newData.DeleteData == "true" ) {
	} else {
	}
        updatedData = userGetInternalInfo(username)
	// if the received password is not the one of the end user we can't erase it's account
	// might be a browser hack
	if ( !base.CheckPasswordHash(newData.CurrentPassword, updatedData.Password) ) {
        	w.Write([]byte("error password"))
                return false
        }	

	// Just need to disable the account by unactivating it
	// It could be recovered by resetting the password
	updatedData.Active = 0
	c, _ := json.Marshal(updatedData)
	base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+updatedData.Nickname,c,"application/json")
	
        // And return positively
        return true
}

func getSessionID(username string) (string) {
	// We need to save the cookie into the user database (TODO)
	// Is the user into the cache
	for _, entry := range cache {
		if ( entry.Nickname  == username ) {
               		if ( entry.Expire.After(time.Now()) ) {
				// Ok the Cookie is not expired
				// We can return it and extend the lifecycle
				entry.Expire = time.Now().Add(time.Second * time.Duration(base.MaxAge))
				return(entry.Cookie)
                       }
		}
        }
	
	// ok we must add an entry

	var newEntry cacheEntry
	newEntry.Nickname = username
	newEntry.Expire = time.Now().Add(time.Second *time.Duration(base.MaxAge) )
	Data := make([]byte, 32)
        io.ReadFull(rand.Reader, Data)
        cookie := base64.URLEncoding.EncodeToString(Data)
	newEntry.Cookie = cookie 
	cache = append(cache, newEntry)
	return(newEntry.Cookie)

}

func getOpenBMC(username string, w http.ResponseWriter) {
	client := &http.Client{}
	var req *http.Request
	req, _ = http.NewRequest("GET","http://"+StorageURI+StorageTCPPORT+"/user/"+username+"/getBMCFirmware", nil)
	response, _  := client.Do(req)
        buf, _ := ioutil.ReadAll(response.Body)
        w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.Write(buf)
}

func getOpenBMCBuildLog(username string, w http.ResponseWriter) {
        client := &http.Client{}
        var req *http.Request
        req, _ = http.NewRequest("GET","http://"+StorageURI+StorageTCPPORT+"/user/"+username+"/getBMCFirmwareBuildLog", nil)
        response, _  := client.Do(req)
        buf, _ := ioutil.ReadAll(response.Body)
        w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
        w.Write(buf)
}


func getLinuxBoot(username string, w http.ResponseWriter) {
        client := &http.Client{}
        var req *http.Request
        req, _ = http.NewRequest("GET","http://"+StorageURI+StorageTCPPORT+"/user/"+username+"/getFirmware", nil)
        response, _  := client.Do(req)
        buf, _ := ioutil.ReadAll(response.Body)
        w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.Write(buf)
}

func getLinuxBootBuildLog(username string, w http.ResponseWriter) {
        client := &http.Client{}
        var req *http.Request
        req, _ = http.NewRequest("GET","http://"+StorageURI+StorageTCPPORT+"/user/"+username+"/getFirmwareBuildLog", nil)
        response, _  := client.Do(req)
        buf, _ := ioutil.ReadAll(response.Body)
        w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
        w.Write(buf)
}

func userCallback(w http.ResponseWriter, r *http.Request) {
        var username,command  string

	path := strings.Split( r.URL.Path, "/" )
	if ( len(path) < 3 ) {
                http.Error(w, "401 Malformed URI", 401)
		return
	}
	username = path[2]
	if ( len(path) >= 4 ) {
		command = path[3]
	}
        switch r.Method {
                case http.MethodGet:
			switch command {
			case "validateUser":
				// got a validation link ....
				// we have to accept user activation
				// First check if the account exist
				// if yes we must get the data, compare the link and if a match
				// activate the user allowing a call to the API to get the connection token
				if ( !validateUser(username, path[4]) ) {
					http.Error(w, "401 Validation string error", 401)
				} else {
						// We just need to display the login page
						// One of the issue is that it is a dynamic page
						// We need to do it through the app.js
						// and load the script in a way it can detect the redirection
						http.Redirect(
    							w, r,
					        	"https://"+r.Host+"/ci/?loginValidated=1",
					        	http.StatusMovedPermanently,
    						)
				}
			case "resetPassword":
				// We have to validate the user, then display the right return page
				if ( !validateUser(username, path[4]) ) {
                                        http.Error(w, "401 Validation string error", 401)
                                } else {
						print("REDIRECTION");
                                                http.Redirect(
                                                        w, r,
                                                        "https://"+r.Host+"/ci/?resetPassword=1&username="+username+"&validation="+path[4],
                                                        http.StatusMovedPermanently,
                                                )
                                }
			case "userGetInternalInfo":
				var result *base.User
                                // Serve the resource.
				fmt.Printf("Requesting %s\n", username)
                                result=userGetInternalInfo(username)
                                b, _ := json.Marshal(*result)
                                fmt.Fprint(w, string(b))
			case "userGetInfo":
				var result *userPublic
                                // Serve the resource.
                                result=userGetInfo(username)
                                b, _ := json.Marshal(*result)
                                fmt.Fprint(w, string(b))

			case "getAvatar":
				getAvatar(username, &w)
			case "getOpenBMC":
				getOpenBMC(username, w)
			case "getLinuxBoot":
				getLinuxBoot(username, w)
                        case "getOpenBMCLog":
                                getOpenBMCBuildLog(username, w)
                        case "getLinuxBootLog":
                                getLinuxBootBuildLog(username, w)
			default:
			}
                case http.MethodPut:
			// Update an existing record.
			switch command {
                        case "updateAvatar":
				updateAvatar(username, w, r)
			case "updateAccount":
				updateAccount(username, w, r)
			default:
				http.Error(w, "401 Unknown user command", 401)
				return
			}
		case http.MethodPost:
			// Ok I am getting there the various parameters to log a user
			switch command {
				case "getToken":
					// We must get the user info and validate the password sent
					// if the user doesn't have any API Token
					// we have to generate it !
					// if the user doesn't exist we need to deny the request
					password := r.FormValue("password")
					var result *base.User
					result=userGetInternalInfo(username)
					if ( !base.CheckPasswordHash(password, result.Password) ) {
						http.Error(w, "401 Password error", 401)
						return
					}
					if ( result.Active == 0 ) {
						http.Error(w, "401 User not activated Please check email", 401)
						return
					}
					// We have the right password !
					// So, we need to send the secret and access token
					// as the end user could login the to the API
					// and load the right page !
					returnValue := " { \"accessKey\" : \""+ result.TokenAuth + 
							"\", \"secretKey\" : \""+ result.TokenSecret + "\" }"
					result.Lastlogin=string(time.Now().Format(time.RFC1123Z))
					b, _ := json.Marshal(result)
				        base.HTTPPutRequest("http://"+StorageURI+StorageTCPPORT+"/user/"+result.Nickname,b,"application/json")

					// As the user might be willing to use OpenBMC we need to send him also a SESSION ID cookie
					// which will be the only way to track him/her as we eveolve from a single app web base
					// platform to a multiple one (our website and the OpenBMC one)
					sessionid := getSessionID(result.Nickname)
					// We need to send back the cookie to the client
					cookie := http.Cookie{Name: "osfci_cookie", Value: sessionid, Path: "/", HttpOnly: true, MaxAge: int(base.MaxAge)}
				        http.SetCookie(w, &cookie)
					fmt.Fprintf(w,string(returnValue))
				case "createUser":
					createUser(username, w, r)
				case "generatePasswordLnkRst":
					sendPasswordResetLink( username, w, r)
				case "resetPassword":
					resetPassword( username, w, r)
				default:
					http.Error(w, "401 Unknown user command\n", 401)

			}
                case http.MethodDelete:
			// Remove the record.
			deleteUser(username, w, r)
                default:
			http.Error(w, "401 Unknown request\n", 401)
        }
}
func main() {
    // http to https redirection
    print("=============================== \n")
    print("| Starting user credentials  |\n")
    print("| Development version -       |\n")
    print("| Private use only            |\n")
    print("=============================== \n")

    mux := http.NewServeMux()
    var CredentialURI = os.Getenv("CREDENTIALS_TCPPORT")
    print("Attaching to "+CredentialURI+"\n")
    // Serve one page site dynamic pages
    mux.HandleFunc("/user/", userCallback)
    log.Fatal(http.ListenAndServe(CredentialURI, mux))
}
