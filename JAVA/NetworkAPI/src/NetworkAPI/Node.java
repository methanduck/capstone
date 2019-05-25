package NetworkAPI;


public class Node {
    //Variables
    public boolean     initialized; //already configured node
    private String      Password;
    private String      IPAddr;
    private String      HostName;
    private String      ModeAuto;
    private String      Oper;
    public int         Temp;
    public int         Humidity;
    public boolean     Light;
    public int         Gas;

    public Node(String password, String IP_Addr, String hostName) {
        Password = password;
        this.IPAddr = IP_Addr;
        HostName = hostName;
    }

    public Node(String IP_Addr) {
        this.IPAddr = IP_Addr;
    }

    public String getPassword() {
        return Password;
    }

    public String getIPAddr() {
        return IPAddr;
    }

    public String getHostName() {
        return HostName;
    }

    public String getOper() {
        return Oper;
    }
    
    public boolean isInitialized() {
        return initialized;
    }

    public void setPassword(String password) {
        Password = password;
    }

    public void setIPAddr(String IPAddr) {
        this.IPAddr = IPAddr;
    }

    public void setHostName(String hostName) {
        HostName = hostName;
    }
}
