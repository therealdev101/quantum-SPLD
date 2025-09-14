require("@nomiclabs/hardhat-ethers");

module.exports = {
  solidity: {
    compilers: [
      {
        version: "0.8.17",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200
          },
        },
      }
    ],
  },
  
  defaultNetwork: "hardhat",
  networks: {
    hardhat: {}
  }
};
